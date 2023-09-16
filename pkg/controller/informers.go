package controller

import (
	"context"
	"fmt"

	"github.com/openfaas/faas-netes/pkg/handlers"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	v1apps "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

func RegisterEventHandlers(statefulsetInformer v1apps.StatefulSetInformer, kubeClient *kubernetes.Clientset, namespace string) {
	statefulsetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			statefulset, ok := obj.(*appsv1.StatefulSet)
			if !ok || statefulset == nil {
				return
			}
			if err := applyValidation(statefulset, kubeClient); err != nil {
				klog.Info(err)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			statefulset, ok := newObj.(*appsv1.StatefulSet)
			if !ok || statefulset == nil {
				return
			}
			if err := applyValidation(statefulset, kubeClient); err != nil {
				klog.Info(err)
			}
		},
	})

	list, err := statefulsetInformer.Lister().StatefulSets(namespace).List(labels.Everything())
	if err != nil {
		klog.Info(err)
		return
	}

	for _, statefulset := range list {
		if err := applyValidation(statefulset, kubeClient); err != nil {
			klog.Info(err)
		}
	}
}

func applyValidation(statefulset *appsv1.StatefulSet, kubeClient *kubernetes.Clientset) error {
	if statefulset.Spec.Replicas == nil {
		return nil
	}

	if _, ok := statefulset.Spec.Template.Labels["faas_function"]; !ok {
		return nil
	}

	current := *statefulset.Spec.Replicas
	var target int
	if current == 0 {
		target = 1
	} else if current > handlers.MaxReplicas {
		target = handlers.MaxReplicas
	} else {
		return nil
	}
	clone := statefulset.DeepCopy()

	value := int32(target)
	clone.Spec.Replicas = &value

	if _, err := kubeClient.AppsV1().StatefulSets(statefulset.Namespace).
		Update(context.Background(), clone, metav1.UpdateOptions{}); err != nil {
		if errors.IsConflict(err) {
			return nil
		}
		return fmt.Errorf("error scaling %s to %d replicas: %w", statefulset.Name, value, err)
	}

	return nil
}
