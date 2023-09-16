package controller

import (
	"fmt"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	faasv1 "github.com/openfaas/faas-netes/pkg/apis/openfaas/v1"
)

func Test_UpdateSecrets_DoesNotAddVolumeIfRequestSecretsIsNil(t *testing.T) {
	request := &faasv1.Function{
		Spec: faasv1.FunctionSpec{
			Name:    "testfunc",
			Secrets: nil,
		},
	}
	existingSecrets := map[string]*corev1.Secret{
		"pullsecret": {Type: corev1.SecretTypeDockercfg},
		"testsecret": {Type: corev1.SecretTypeOpaque, Data: map[string][]byte{"filename": []byte("contents")}},
	}

	statefulset := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "testfunc", Image: "alpine:latest"},
					},
				},
			},
		},
	}
	err := UpdateSecrets(request, statefulset, existingSecrets)
	if err != nil {
		t.Errorf("unexpected error %s", err.Error())
	}

	validateEmptySecretVolumesAndMounts(t, statefulset)

}

func Test_UpdateSecrets_DoesNotAddVolumeIfRequestSecretsIsEmpty(t *testing.T) {
	request := &faasv1.Function{
		Spec: faasv1.FunctionSpec{
			Name:    "testfunc",
			Secrets: []string{},
		},
	}
	existingSecrets := map[string]*corev1.Secret{
		"pullsecret": {Type: corev1.SecretTypeDockercfg},
		"testsecret": {Type: corev1.SecretTypeOpaque, Data: map[string][]byte{"filename": []byte("contents")}},
	}

	statefulset := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "testfunc", Image: "alpine:latest"},
					},
				},
			},
		},
	}
	err := UpdateSecrets(request, statefulset, existingSecrets)
	if err != nil {
		t.Errorf("unexpected error %s", err.Error())
	}

	validateEmptySecretVolumesAndMounts(t, statefulset)

}

func Test_UpdateSecrets_RemovesAllCopiesOfExitingSecretsVolumes(t *testing.T) {
	volumeName := "testfunc-projected-secrets"
	request := &faasv1.Function{
		Spec: faasv1.FunctionSpec{
			Name:    "testfunc",
			Secrets: []string{},
		},
	}
	existingSecrets := map[string]*corev1.Secret{
		"pullsecret": {Type: corev1.SecretTypeDockercfg},
		"testsecret": {Type: corev1.SecretTypeOpaque, Data: map[string][]byte{"filename": []byte("contents")}},
	}

	statefulset := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "testfunc",
							Image: "alpine:latest",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name: volumeName,
								},
								{
									Name: volumeName,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: volumeName,
						},
						{
							Name: volumeName,
						},
					},
				},
			},
		},
	}
	err := UpdateSecrets(request, statefulset, existingSecrets)
	if err != nil {
		t.Errorf("unexpected error %s", err.Error())
	}

	validateEmptySecretVolumesAndMounts(t, statefulset)

}

func Test_UpdateSecrets_AddNewSecretVolume(t *testing.T) {
	request := &faasv1.Function{
		Spec: faasv1.FunctionSpec{
			Name:    "testfunc",
			Secrets: []string{"pullsecret", "testsecret"},
		},
	}
	existingSecrets := map[string]*corev1.Secret{
		"pullsecret": {Type: corev1.SecretTypeDockercfg},
		"testsecret": {Type: corev1.SecretTypeOpaque, Data: map[string][]byte{"filename": []byte("contents")}},
	}

	statefulset := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "testfunc", Image: "alpine:latest"},
					},
				},
			},
		},
	}
	err := UpdateSecrets(request, statefulset, existingSecrets)
	if err != nil {
		t.Errorf("unexpected error %s", err.Error())
	}

	validateNewSecretVolumesAndMounts(t, statefulset)

}

func Test_UpdateSecrets_ReplacesPreviousSecretMountWithNewMount(t *testing.T) {
	request := &faasv1.Function{
		Spec: faasv1.FunctionSpec{
			Name:    "testfunc",
			Secrets: []string{"pullsecret", "testsecret"},
		},
	}
	existingSecrets := map[string]*corev1.Secret{
		"pullsecret": {Type: corev1.SecretTypeDockercfg},
		"testsecret": {Type: corev1.SecretTypeOpaque, Data: map[string][]byte{"filename": []byte("contents")}},
	}

	statefulset := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "testfunc", Image: "alpine:latest"},
					},
				},
			},
		},
	}
	err := UpdateSecrets(request, statefulset, existingSecrets)
	if err != nil {
		t.Errorf("unexpected error %s", err.Error())
	}
	// mimic the statefulset already existing and deployed with the same secrets by running
	// UpdateSecrets twice, the first run represents the original statefulset, the second run represents
	// retrieving the statefulset from the k8s api and applying the update to it
	err = UpdateSecrets(request, statefulset, existingSecrets)
	if err != nil {
		t.Errorf("unexpected error %s", err.Error())
	}

	validateNewSecretVolumesAndMounts(t, statefulset)

}

func Test_UpdateSecrets_RemovesSecretsVolumeIfRequestSecretsIsEmptyOrNil(t *testing.T) {
	request := &faasv1.Function{
		Spec: faasv1.FunctionSpec{
			Name:    "testfunc",
			Secrets: []string{"pullsecret", "testsecret"},
		},
	}
	existingSecrets := map[string]*corev1.Secret{
		"pullsecret": {Type: corev1.SecretTypeDockercfg},
		"testsecret": {Type: corev1.SecretTypeOpaque, Data: map[string][]byte{"filename": []byte("contents")}},
	}

	statefulset := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "testfunc", Image: "alpine:latest"},
					},
				},
			},
		},
	}
	err := UpdateSecrets(request, statefulset, existingSecrets)
	if err != nil {
		t.Errorf("unexpected error %s", err.Error())
	}

	validateNewSecretVolumesAndMounts(t, statefulset)

	request = &faasv1.Function{
		Spec: faasv1.FunctionSpec{
			Name:    "testfunc",
			Secrets: []string{},
		},
	}
	err = UpdateSecrets(request, statefulset, existingSecrets)
	if err != nil {
		t.Errorf("unexpected error %s", err.Error())
	}

	validateEmptySecretVolumesAndMounts(t, statefulset)
}

func validateEmptySecretVolumesAndMounts(t *testing.T, statefulset *appsv1.StatefulSet) {
	numVolumes := len(statefulset.Spec.Template.Spec.Volumes)
	if numVolumes != 0 {
		fmt.Printf("%+v", statefulset.Spec.Template.Spec.Volumes)
		t.Errorf("Incorrect number of volumes: expected 0, got %d", numVolumes)
	}

	c := statefulset.Spec.Template.Spec.Containers[0]
	numVolumeMounts := len(c.VolumeMounts)
	if numVolumeMounts != 0 {
		t.Errorf("Incorrect number of volumes mounts: expected 0, got %d", numVolumeMounts)
	}
}

func validateNewSecretVolumesAndMounts(t *testing.T, statefulset *appsv1.StatefulSet) {
	numVolumes := len(statefulset.Spec.Template.Spec.Volumes)
	if numVolumes != 1 {
		t.Errorf("Incorrect number of volumes: expected 1, got %d", numVolumes)
	}

	volume := statefulset.Spec.Template.Spec.Volumes[0]
	if volume.Name != "testfunc-projected-secrets" {
		t.Errorf("Incorrect volume name: expected \"testfunc-projected-secrets\", got \"%s\"", volume.Name)
	}

	if volume.VolumeSource.Projected == nil {
		t.Error("Secrets volume is not a projected volume type")
	}

	if volume.VolumeSource.Projected.Sources[0].Secret.Items[0].Key != "filename" {
		t.Error("Project secret not constructed correctly")
	}

	c := statefulset.Spec.Template.Spec.Containers[0]
	numVolumeMounts := len(c.VolumeMounts)
	if numVolumeMounts != 1 {
		t.Errorf("Incorrect number of volumes mounts: expected 1, got %d", numVolumeMounts)
	}

	mount := c.VolumeMounts[0]
	if mount.Name != "testfunc-projected-secrets" {
		t.Errorf("Incorrect volume mounts: expected \"testfunc-projected-secrets\", got \"%s\"", mount.Name)
	}

	if mount.MountPath != secretsMountPath {
		t.Errorf("Incorrect volume mount path: expected \"%s\", got \"%s\"", secretsMountPath, mount.MountPath)
	}
}
