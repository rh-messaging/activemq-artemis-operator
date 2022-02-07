package common

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
)

// extra kinds
const (
	RouteKind              = "Route"
	OpenShiftAPIServerKind = "OpenShiftAPIServer"
)

func compareQuantities(resList1 corev1.ResourceList, resList2 corev1.ResourceList, keys []corev1.ResourceName) bool {

	for _, key := range keys {
		if q1, ok1 := resList1[key]; ok1 {
			if q2, ok2 := resList2[key]; ok2 {
				if q1.Cmp(q2) != 0 {
					return false
				}
			} else {
				return false
			}
		} else {
			if _, ok2 := resList2[key]; ok2 {
				return false
			}
		}
	}
	return true
}

func CompareRequiredResources(res1 *corev1.ResourceRequirements, res2 *corev1.ResourceRequirements) bool {

	resNames := []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory, corev1.ResourceStorage, corev1.ResourceEphemeralStorage}
	if !compareQuantities(res1.Limits, res2.Limits, resNames) {
		return false
	}

	if !compareQuantities(res1.Requests, res2.Requests, resNames) {
		return false
	}
	return true
}

func ToJson(obj interface{}) (string, error) {
	bytes, err := json.Marshal(obj)
	if err == nil {
		return string(bytes), nil
	}
	return "", err
}

func FromJson(jsonStr *string, obj interface{}) error {
	return json.Unmarshal([]byte(*jsonStr), obj)
}
