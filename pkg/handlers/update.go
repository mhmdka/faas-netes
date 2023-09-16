// Copyright 2020 OpenFaaS Author(s)
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/openfaas/faas-netes/pkg/k8s"

	types "github.com/openfaas/faas-provider/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MakeUpdateHandler update specified function
func MakeUpdateHandler(defaultNamespace string, factory k8s.FunctionFactory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if r.Body != nil {
			defer r.Body.Close()
		}

		body, _ := io.ReadAll(r.Body)

		request := types.FunctionDeployment{}
		err := json.Unmarshal(body, &request)
		if err != nil {
			wrappedErr := fmt.Errorf("unable to unmarshal request: %s", err.Error())
			http.Error(w, wrappedErr.Error(), http.StatusBadRequest)
			return
		}

		if err := ValidateDeployRequest(&request); err != nil {
			wrappedErr := fmt.Errorf("validation failed: %s", err.Error())
			http.Error(w, wrappedErr.Error(), http.StatusBadRequest)
			return
		}

		lookupNamespace := defaultNamespace
		if len(request.Namespace) > 0 {
			lookupNamespace = request.Namespace
		}

		if lookupNamespace != defaultNamespace {
			http.Error(w, fmt.Sprintf("namespace must be: %s", defaultNamespace), http.StatusBadRequest)
			return
		}

		if lookupNamespace == "kube-system" {
			http.Error(w, "unable to list within the kube-system namespace", http.StatusUnauthorized)
			return
		}

		annotations, err := buildAnnotations(request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err, status := updateStatefulSetSpec(ctx, lookupNamespace, factory, request, annotations); err != nil {
			if !k8s.IsNotFound(err) {
				log.Printf("error updating StatefulSet: %s.%s, error: %s\n", request.Service, lookupNamespace, err)

				return
			}

			wrappedErr := fmt.Errorf("unable update StatefulSet: %s.%s, error: %s", request.Service, lookupNamespace, err.Error())
			http.Error(w, wrappedErr.Error(), status)
			return
		}

		if err, status := updateService(lookupNamespace, factory, request, annotations); err != nil {
			if !k8s.IsNotFound(err) {
				log.Printf("error updating service: %s.%s, error: %s\n", request.Service, lookupNamespace, err)
			}

			wrappedErr := fmt.Errorf("unable update Service: %s.%s, error: %s", request.Service, request.Namespace, err.Error())
			http.Error(w, wrappedErr.Error(), status)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func updateStatefulSetSpec(
	ctx context.Context,
	functionNamespace string,
	factory k8s.FunctionFactory,
	request types.FunctionDeployment,
	annotations map[string]string) (err error, httpStatus int) {

	getOpts := metav1.GetOptions{}

	statefulset, findDeployErr := factory.Client.AppsV1().
		StatefulSets(functionNamespace).
		Get(context.TODO(), request.Service, getOpts)

	if findDeployErr != nil {
		return findDeployErr, http.StatusNotFound
	}

	if len(statefulset.Spec.Template.Spec.Containers) > 0 {
		statefulset.Spec.Template.Spec.Containers[0].Image = request.Image

		statefulset.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullAlways

		statefulset.Spec.Template.Spec.Containers[0].Env = buildEnvVars(&request)

		factory.ConfigureReadOnlyRootFilesystem(request, statefulset)
		factory.ConfigureContainerUserID(statefulset)

		statefulset.Spec.Template.Spec.NodeSelector = createSelector(request.Constraints)

		labels := map[string]string{
			"faas_function": request.Service,
			"uid":           fmt.Sprintf("%d", time.Now().Nanosecond()),
		}

		if request.Labels != nil {
			if min := getMinReplicaCount(*request.Labels); min != nil {
				statefulset.Spec.Replicas = min
			}

			for k, v := range *request.Labels {
				labels[k] = v
			}
		}

		// statefulset.Labels = labels
		statefulset.Spec.Template.ObjectMeta.Labels = labels

		// store the current annotations so that we can diff the annotations
		// and determine which profiles need to be removed
		currentAnnotations := statefulset.Annotations
		statefulset.Annotations = annotations
		statefulset.Spec.Template.Annotations = annotations
		statefulset.Spec.Template.ObjectMeta.Annotations = annotations

		resources, resourceErr := createResources(request)
		if resourceErr != nil {
			return resourceErr, http.StatusBadRequest
		}

		statefulset.Spec.Template.Spec.Containers[0].Resources = *resources

		secrets := k8s.NewSecretsClient(factory.Client)
		existingSecrets, err := secrets.GetSecrets(functionNamespace, request.Secrets)
		if err != nil {
			return err, http.StatusBadRequest
		}

		err = factory.ConfigureSecrets(request, statefulset, existingSecrets)
		if err != nil {
			log.Println(err)
			return err, http.StatusBadRequest
		}

		probes, err := factory.MakeProbes(request)
		if err != nil {
			return err, http.StatusBadRequest
		}

		statefulset.Spec.Template.Spec.Containers[0].LivenessProbe = probes.Liveness
		statefulset.Spec.Template.Spec.Containers[0].ReadinessProbe = probes.Readiness

		// compare the annotations from args to the cache copy of the statefulset annotations
		// at this point we have already updated the annotations to the new value, if we
		// compare to that it will produce an empty list
		profileNamespace := factory.Config.ProfilesNamespace
		profileList, err := factory.GetProfilesToRemove(ctx, profileNamespace, annotations, currentAnnotations)
		if err != nil {
			return err, http.StatusBadRequest
		}
		for _, profile := range profileList {
			factory.RemoveProfile(profile, statefulset)
		}

		profileList, err = factory.GetProfiles(ctx, profileNamespace, annotations)
		if err != nil {
			return err, http.StatusBadRequest
		}
		for _, profile := range profileList {
			factory.ApplyProfile(profile, statefulset)
		}
	}

	if _, updateErr := factory.Client.AppsV1().
		StatefulSets(functionNamespace).
		Update(context.TODO(), statefulset, metav1.UpdateOptions{}); updateErr != nil {

		return updateErr, http.StatusInternalServerError
	}

	return nil, http.StatusAccepted
}

func updateService(
	functionNamespace string,
	factory k8s.FunctionFactory,
	request types.FunctionDeployment,
	annotations map[string]string) (err error, httpStatus int) {

	getOpts := metav1.GetOptions{}

	service, findServiceErr := factory.Client.CoreV1().
		Services(functionNamespace).
		Get(context.TODO(), request.Service, getOpts)

	if findServiceErr != nil {
		return findServiceErr, http.StatusNotFound
	}

	service.Annotations = annotations

	if _, updateErr := factory.Client.CoreV1().
		Services(functionNamespace).
		Update(context.TODO(), service, metav1.UpdateOptions{}); updateErr != nil {

		return updateErr, http.StatusInternalServerError
	}

	return nil, http.StatusAccepted
}
