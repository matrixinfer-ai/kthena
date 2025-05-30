package utils

import (
	"regexp"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func GetNamespaceName(obj metav1.Object) types.NamespacedName {
	return types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

// inferGroupRegex is a regular expression that extracts the parent modelInfer and ordinal from the Name of an inferGroup
var inferGroupRegex = regexp.MustCompile("(.*)-([0-9]+)$")

// GetParentNameAndOrdinal gets the name of inferGroup's parent modelInfer and inferGroup's ordinal as extracted from its Name.
// For example, the infergroup name is vllm-sample-0, this function can be used to obtain the modelinfer name corresponding to the infergroup,
// which is vllm-sample and the serial number is 0.
func GetParentNameAndOrdinal(groupName string) (string, int) {
	parent := ""
	ordinal := -1
	subMatches := inferGroupRegex.FindStringSubmatch(groupName)
	if len(subMatches) < 3 {
		return parent, ordinal
	}
	parent = subMatches[1]
	if i, err := strconv.ParseInt(subMatches[2], 10, 32); err == nil {
		ordinal = int(i)
	}
	return parent, ordinal
}

func GenerateInferGroupName(miName string, idx int) string {
	return miName + "-" + strconv.Itoa(idx)
}
