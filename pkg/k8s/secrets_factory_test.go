// Copyright 2020 OpenFaaS Author(s)
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package k8s

import (
	"fmt"
	"testing"

	types "github.com/openfaas/faas-provider/types"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_ReadFunctionSecretsSpec(t *testing.T) {

	f := mockFactory()
	existingSecrets := map[string]*apiv1.Secret{
		"pullsecret": {Type: apiv1.SecretTypeDockercfg},
		"testsecret": {Type: apiv1.SecretTypeOpaque, Data: map[string][]byte{"filename": []byte("contents")}},
	}
	functionDep := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "testfunc"},
		Spec: appsv1.StatefulSetSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "testfunc", Image: "alpine:latest"},
					},
				},
			},
		},
	}

	cases := []struct {
		name       string
		req        types.FunctionDeployment
		statefulset appsv1.StatefulSet
		expected   []string
	}{
		{
			name: "empty secrets, returns empty slice",
			req: types.FunctionDeployment{
				Service: "testfunc",
				Secrets: []string{},
			},
			statefulset: functionDep,
			expected:   []string{},
		},
		{
			name: "detects and extracts image pull secret",
			req: types.FunctionDeployment{
				Service: "testfunc",
				Secrets: []string{"pullsecret"},
			},
			statefulset: functionDep,
			expected:   []string{"pullsecret"},
		},
		{
			name: "detects and extracts projected generic secret",
			req: types.FunctionDeployment{
				Service: "testfunc",
				Secrets: []string{"testsecret"},
			},
			statefulset: functionDep,
			expected:   []string{"testsecret"},
		},
		{
			name: "detects and extracts both pull secrets and projected generic secret, result is sorted",
			req: types.FunctionDeployment{
				Service: "testfunc",
				Secrets: []string{"testsecret", "pullsecret"},
			},
			statefulset: functionDep,
			expected:   []string{"pullsecret", "testsecret"},
		},
	}

	for _, tc := range cases {
		err := f.ConfigureSecrets(tc.req, &tc.statefulset, existingSecrets)
		if err != nil {
			t.Fatalf("unexpected error result: got %q", err)
		}

		parsedSecrets := ReadFunctionSecretsSpec(tc.statefulset)
		if len(tc.expected) != len(parsedSecrets) {
			t.Fatalf("incorrect secret count, expected: %v, got: %v", tc.expected, parsedSecrets)
		}

		for idx, expected := range tc.expected {
			value := parsedSecrets[idx]
			if expected != value {
				t.Fatalf("incorrect secret in idx %d, expected: %q, got: %q", idx, expected, value)
			}
		}
	}

}

func Test_FunctionFactory_ConfigureSecrets(t *testing.T) {
	f := mockFactory()
	existingSecrets := map[string]*apiv1.Secret{
		"pullsecret": {Type: apiv1.SecretTypeDockercfg},
		"testsecret": {Type: apiv1.SecretTypeOpaque, Data: map[string][]byte{"filename": []byte("contents")}},
	}

	basicStatefulset := appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "testfunc", Image: "alpine:latest"},
					},
				},
			},
		},
	}

	volumeName := "testfunc-projected-secrets"
	withExistingSecret := appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  "testfunc",
							Image: "alpine:latest",
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name: volumeName,
								},
								{
									Name: volumeName,
								},
							},
						},
					},
					Volumes: []apiv1.Volume{
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

	cases := []struct {
		name       string
		req        types.FunctionDeployment
		statefulset appsv1.StatefulSet
		validator  func(t *testing.T, statefulset *appsv1.StatefulSet)
		err        error
	}{
		{
			name: "does not add volume if request secrets is nil",
			req: types.FunctionDeployment{
				Service: "testfunc",
				Secrets: nil,
			},
			statefulset: basicStatefulset,
			validator:  validateEmptySecretVolumesAndMounts,
		},
		{
			name: "does not add volume if request secrets is nil",
			req: types.FunctionDeployment{
				Service: "testfunc",
				Secrets: []string{},
			},
			statefulset: basicStatefulset,
			validator:  validateEmptySecretVolumesAndMounts,
		},
		{
			name: "removes all copies of exiting secrets volumes",
			req: types.FunctionDeployment{
				Service: "testfunc",
				Secrets: []string{},
			},
			statefulset: withExistingSecret,
			validator:  validateEmptySecretVolumesAndMounts,
		},
		{
			name: "add new secret volume",
			req: types.FunctionDeployment{
				Service: "testfunc",
				Secrets: []string{"pullsecret", "testsecret"},
			},
			statefulset: basicStatefulset,
			validator:  validateNewSecretVolumesAndMounts,
		},
		{
			name: "replaces previous secret mount with new mount",
			req: types.FunctionDeployment{
				Service: "testfunc",
				Secrets: []string{"pullsecret", "testsecret"},
			},
			statefulset: withExistingSecret,
			validator:  validateNewSecretVolumesAndMounts,
		},
		{
			name: "removes secrets volume if request secrets is empty or nil",
			req: types.FunctionDeployment{
				Service: "testfunc",
				Secrets: []string{},
			},
			statefulset: withExistingSecret,
			validator:  validateEmptySecretVolumesAndMounts,
		},
	}

	for _, tc := range cases {
		err := f.ConfigureSecrets(tc.req, &tc.statefulset, existingSecrets)
		if err != tc.err {
			t.Errorf("unexpected error result: got %v, expected %v", err, tc.err)
		}

		tc.validator(t, &tc.statefulset)
	}
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
