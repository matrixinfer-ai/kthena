package utils

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	workloadv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
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

func generateEntryPodName(groupName, roleName string, roleIndex int) string {
	// entry-pod number starts from 0
	// For example, EntryPodName is vllm-sample-0-prefill-1-0, represents the entry-pod in the second replica of the prefill role
	return groupName + "-" + roleName + "-" + strconv.Itoa(roleIndex) + "-" + "0"
}

func generateWorkerPodName(groupName, roleName string, roleIndex, podIndex int) string {
	// worker-pod number starts from 1
	// For example, WorkerPodName is vllm-sample-0-prefill-1-1, represents the first worker-pod in the second replica of the prefill role
	return groupName + "-" + roleName + "-" + strconv.Itoa(roleIndex) + "-" + strconv.Itoa(podIndex)
}

func GenerateEntryPod(role workloadv1alpha1.Role, mi *workloadv1alpha1.ModelInfer, groupName string, roleIndex int) *corev1.Pod {
	entryPodName := generateEntryPodName(groupName, role.Name, roleIndex)
	entryPod := createBasePod(role, mi, entryPodName, groupName)
	entryPod.Spec.Hostname = entryPodName
	entryPod.Spec.Subdomain = entryPodName
	// Build environment variables into each container of all pod
	envVars := createCommonEnvVars(role, mi, entryPod, 0)
	addPodEnvVars(entryPod, envVars...)
	return entryPod
}

func GenerateWorkerPod(role workloadv1alpha1.Role, mi *workloadv1alpha1.ModelInfer, entryPod *corev1.Pod, groupName string, roleIndex, podIndex int) *corev1.Pod {
	workerPodName := generateWorkerPodName(groupName, role.Name, roleIndex, podIndex)
	workerPod := createBasePod(role, mi, workerPodName, groupName)
	// Build environment variables into each container of all pod
	envVars := createCommonEnvVars(role, mi, entryPod, podIndex)
	addPodEnvVars(workerPod, envVars...)
	return workerPod
}

func createBasePod(role workloadv1alpha1.Role, mi *workloadv1alpha1.ModelInfer, name, groupName string) *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: mi.Namespace,
			Labels: map[string]string{
				workloadv1alpha1.ModelInferNameLabelKey: mi.Name,
				workloadv1alpha1.GroupNameLabelKey:      groupName,
				workloadv1alpha1.RoleLabelKey:           role.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				newModelInferOwnerRef(mi),
			},
		},
	}
}

func createCommonEnvVars(role workloadv1alpha1.Role, mi *workloadv1alpha1.ModelInfer, entryPod *corev1.Pod, workerIndex int) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  workloadv1alpha1.GroupSizeEnv,
			Value: strconv.Itoa(int(*role.WorkerReplicas) + 1),
		},
		{
			Name:  workloadv1alpha1.EntryAddressEnv,
			Value: fmt.Sprintf("%s.%s.%s", entryPod.Spec.Hostname, entryPod.Spec.Subdomain, mi.Namespace),
		},
		{
			Name:  workloadv1alpha1.WorkerIndexEnv,
			Value: strconv.Itoa(workerIndex),
		},
	}
}

// addPodEnvVars adds new env vars to the container.
func addPodEnvVars(pod *corev1.Pod, newEnvVars ...corev1.EnvVar) {
	if pod == nil {
		return
	}

	for i := range pod.Spec.Containers {
		addEnvVars(&pod.Spec.Containers[i], newEnvVars...)
	}

	for i := range pod.Spec.InitContainers {
		addEnvVars(&pod.Spec.InitContainers[i], newEnvVars...)
	}
}

func addEnvVars(container *corev1.Container, newEnvVars ...corev1.EnvVar) {
	if container == nil {
		return
	}
	// Used to quickly find whether the newly added environment variable already exists
	newEnvMap := make(map[string]struct{})
	for _, env := range newEnvVars {
		newEnvMap[env.Name] = struct{}{}
	}

	// Collect environment variables that need to be retained
	var retainedEnvVars []corev1.EnvVar
	for _, env := range container.Env {
		if _, exists := newEnvMap[env.Name]; !exists {
			// This environment variable does not need to be updated.
			retainedEnvVars = append(retainedEnvVars, env)
		}
	}
	// Merge existing variables that are retained and newly added variables
	container.Env = append(retainedEnvVars, newEnvVars...)
}

// newModelInferOwnerRef creates an OwnerReference pointing to the given ModelInfer.
func newModelInferOwnerRef(mi *workloadv1alpha1.ModelInfer) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         workloadv1alpha1.ModelInferKind.GroupVersion().String(),
		Kind:               workloadv1alpha1.ModelInferKind.Kind,
		Name:               mi.Name,
		UID:                mi.UID,
		BlockOwnerDeletion: ptr.To(true),
		Controller:         ptr.To(true),
	}
}

func CreateHeadlessService(ctx context.Context, k8sClient kubernetes.Interface, mi *workloadv1alpha1.ModelInfer, serviceName string, serviceSelector map[string]string) error {
	headlessService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: mi.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				newModelInferOwnerRef(mi),
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP:                "None", // defines service as headless
			Selector:                 serviceSelector,
			PublishNotReadyAddresses: true,
		},
	}
	// create the service in the cluster
	klog.V(2).Info("Creating headless service")
	_, err := k8sClient.CoreV1().Services(mi.Namespace).Create(ctx, &headlessService, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create headless service failed: %v", err)
	}
	return nil
}
