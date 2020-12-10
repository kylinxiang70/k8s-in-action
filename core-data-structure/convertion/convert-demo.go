package main

import (
	"fmt"

	asv1 "k8s.io/api/autoscaling/v1"
	asv2beta1 "k8s.io/api/autoscaling/v2beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	asinternal "k8s.io/kubernetes/pkg/apis/autoscaling"
)

func main() {
	fmt.Println("start")
	scheme := runtime.NewScheme()

	scheme.AddKnownTypes(asv1.SchemeGroupVersion, &asv1.HorizontalPodAutoscaler{})
	scheme.AddKnownTypes(asv2beta1.SchemeGroupVersion, &asv2beta1.HorizontalPodAutoscaler{})
	scheme.AddKnownTypes(asinternal.SchemeGroupVersion, &asv2beta1.HorizontalPodAutoscaler{})

	metav1.AddToGroupVersion(scheme, asv1.SchemeGroupVersion)
	metav1.AddToGroupVersion(scheme, asv2beta1.SchemeGroupVersion)
	//metav1.AddToGroupVersion(scheme, asinternal.SchemeGroupVersion)

	v1hpa := &asv1.HorizontalPodAutoscaler{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HorizontalPodAutoscaler",
			APIVersion: "autoscaling/v1",
		},
	}

	// v1 -->  internal
	hpaInternal, err := scheme.ConvertToVersion(v1hpa, asinternal.SchemeGroupVersion)
	if err != nil {
		panic(err)
	}

	fmt.Println("GVK: ", hpaInternal.GetObjectKind().GroupVersionKind().String())
}
