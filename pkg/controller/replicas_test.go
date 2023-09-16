package controller

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes/fake"

	faasv1 "github.com/openfaas/faas-netes/pkg/apis/openfaas/v1"
	"github.com/openfaas/faas-netes/pkg/k8s"
)

func Test_Replicas(t *testing.T) {
	scenarios := []struct {
		name     string
		function *faasv1.Function
		deploy   *appsv1.StatefulSet
		expected *int32
	}{
		{
			"return nil replicas when label is missing and statefulset does not exist",
			&faasv1.Function{},
			nil,
			nil,
		},
		{
			"return nil replicas when label is missing and statefulset has no replicas",
			&faasv1.Function{},
			&appsv1.StatefulSet{},
			nil,
		},
		{
			"return min replicas when label is present and statefulset has nil replicas",
			&faasv1.Function{Spec: faasv1.FunctionSpec{Labels: &map[string]string{LabelMinReplicas: "2"}}},
			&appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Replicas: nil}},
			int32p(2),
		},
		{
			"return min replicas when label is present and statefulset has replicas less than min",
			&faasv1.Function{Spec: faasv1.FunctionSpec{Labels: &map[string]string{LabelMinReplicas: "2"}}},
			&appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Replicas: int32p(1)}},
			int32p(2),
		},
		{
			"return existing replicas when label is present and statefulset has more replicas than min",
			&faasv1.Function{Spec: faasv1.FunctionSpec{Labels: &map[string]string{LabelMinReplicas: "2"}}},
			&appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Replicas: int32p(3)}},
			int32p(3),
		},
		{
			"return existing replicas when label is missing and statefulset has replicas set by HPA",
			&faasv1.Function{Spec: faasv1.FunctionSpec{}},
			&appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Replicas: int32p(3)}},
			int32p(3),
		},
		{
			"return zero replicas when label is present and statefulset has zero replicas",
			&faasv1.Function{Spec: faasv1.FunctionSpec{Labels: &map[string]string{LabelMinReplicas: "2"}}},
			&appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Replicas: int32p(0)}},
			int32p(0),
		},
	}

	factory := NewFunctionFactory(fake.NewSimpleClientset(),
		k8s.DeploymentConfig{
			LivenessProbe:  &k8s.ProbeConfig{},
			ReadinessProbe: &k8s.ProbeConfig{},
		})

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			deploy := newStatefulSet(s.function, s.deploy, nil, factory)
			value := deploy.Spec.Replicas

			if s.expected != nil && value != nil {
				if *s.expected != *value {
					t.Errorf("incorrect replica count: expected %v, got %v", *s.expected, *value)
				}
			} else if s.expected != value {
				t.Errorf("incorrect replica count: expected %v, got %v", s.expected, value)
			}
		})
	}
}
