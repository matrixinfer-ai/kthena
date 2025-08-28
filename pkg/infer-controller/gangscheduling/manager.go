/*
Copyright MatrixInfer-AI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gangscheduling

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	schedulingv1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"
	volcanoclient "volcano.sh/apis/pkg/client/clientset/versioned"

	workloadv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
)

const (
	// VolcanoGroupNameAnnotation is the annotation key for volcano group name
	VolcanoGroupNameAnnotation = "volcano.sh/group-name"
	// GangLevelLabelKey is the label key for gang scheduling level
	GangLevelLabelKey = "modelinfer.matrixinfer.ai/gang-level"
	// CreatedByAnnotation is the annotation key for created by
	CreatedByAnnotation = "modelinfer.matrixinfer.ai/created-by"
	// CreatedByValue is the annotation value for created by modelinfer controller
	CreatedByValue = "modelinfer-controller"
	// DefaultTTLSecondsAfterFinished is the default TTL for PodGroup after finished
	DefaultTTLSecondsAfterFinished = 3600 // 1 hour
)

// Manager manages PodGroups for gang scheduling
type Manager struct {
	kubeClient    kubernetes.Interface
	volcanoClient volcanoclient.Interface
}

// NewManager creates a new gang scheduling manager
func NewManager(kubeClient kubernetes.Interface, volcanoClient volcanoclient.Interface) Manager {
	return Manager{
		kubeClient:    kubeClient,
		volcanoClient: volcanoClient,
	}
}

// ManagePodGroups manages PodGroups for a ModelInfer instance
func (m *Manager) ManagePodGroups(ctx context.Context, mi *workloadv1alpha1.ModelInfer) error {
	if !m.isGangSchedulingEnabled(mi) {
		// Gang scheduling is disabled, clean up any existing PodGroups
		return m.cleanupPodGroups(ctx, mi)
	}

	switch mi.Spec.Template.GangSchedule.Level {
	case workloadv1alpha1.GangScheduleLevelGroup:
		return m.manageGroupLevelPodGroups(ctx, mi)
	case workloadv1alpha1.GangScheduleLevelRole:
		return m.manageRoleLevelPodGroups(ctx, mi)
	default:
		// Default to group level
		return m.manageGroupLevelPodGroups(ctx, mi)
	}
}

// isGangSchedulingEnabled checks if gang scheduling is enabled for the ModelInfer
func (m *Manager) isGangSchedulingEnabled(mi *workloadv1alpha1.ModelInfer) bool {
	if mi.Spec.Template.GangSchedule.Enable == nil {
		return true // Default is true
	}
	return *mi.Spec.Template.GangSchedule.Enable
}

// manageGroupLevelPodGroups manages PodGroups for group-level gang scheduling
func (m *Manager) manageGroupLevelPodGroups(ctx context.Context, mi *workloadv1alpha1.ModelInfer) error {
	expectedReplicas := int(*mi.Spec.Replicas)

	// Get existing PodGroups
	existingPodGroups, err := m.getExistingPodGroups(ctx, mi)
	if err != nil {
		return fmt.Errorf("failed to get existing PodGroups: %v", err)
	}

	// Create or update PodGroups for each InferGroup
	for i := 0; i < expectedReplicas; i++ {
		podGroupName := m.generateGroupLevelPodGroupName(mi.Name, i)

		if existingPG, exists := existingPodGroups[podGroupName]; exists {
			// Update existing PodGroup if needed
			if err := m.updatePodGroupIfNeeded(ctx, existingPG, mi, i); err != nil {
				return fmt.Errorf("failed to update PodGroup %s: %v", podGroupName, err)
			}
		} else {
			// Create new PodGroup
			if err := m.createGroupLevelPodGroup(ctx, mi, i); err != nil {
				return fmt.Errorf("failed to create PodGroup %s: %v", podGroupName, err)
			}
		}
	}

	// Clean up excess PodGroups
	return m.cleanupExcessPodGroups(ctx, mi, existingPodGroups, expectedReplicas)
}

// manageRoleLevelPodGroups manages PodGroups for role-level gang scheduling
func (m *Manager) manageRoleLevelPodGroups(ctx context.Context, mi *workloadv1alpha1.ModelInfer) error {
	expectedReplicas := int(*mi.Spec.Replicas)

	// Get existing PodGroups
	existingPodGroups, err := m.getExistingPodGroups(ctx, mi)
	if err != nil {
		return fmt.Errorf("failed to get existing PodGroups: %v", err)
	}

	// Create or update PodGroups for each InferGroup (same naming as group level)
	for i := 0; i < expectedReplicas; i++ {
		podGroupName := m.generateGroupLevelPodGroupName(mi.Name, i)

		if existingPG, exists := existingPodGroups[podGroupName]; exists {
			// Update existing PodGroup if needed
			if err := m.updateRoleLevelPodGroupIfNeeded(ctx, existingPG, mi, i); err != nil {
				return fmt.Errorf("failed to update PodGroup %s: %v", podGroupName, err)
			}
		} else {
			// Create new PodGroup
			if err := m.createRoleLevelPodGroup(ctx, mi, i); err != nil {
				return fmt.Errorf("failed to create PodGroup %s: %v", podGroupName, err)
			}
		}
	}

	// Clean up excess PodGroups
	return m.cleanupExcessPodGroups(ctx, mi, existingPodGroups, expectedReplicas)
}

// createGroupLevelPodGroup creates a PodGroup for group-level gang scheduling
func (m *Manager) createGroupLevelPodGroup(ctx context.Context, mi *workloadv1alpha1.ModelInfer, inferGroupIndex int) error {
	podGroupName := m.generateGroupLevelPodGroupName(mi.Name, inferGroupIndex)

	// Calculate total pods and resources for this InferGroup
	minMember, minTaskMember, minResources := m.calculateGroupLevelRequirements(mi)

	podGroup := &schedulingv1beta1.PodGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podGroupName,
			Namespace: mi.Namespace,
			Labels: map[string]string{
				workloadv1alpha1.ModelInferNameLabelKey: mi.Name,
				workloadv1alpha1.GroupNameLabelKey:      podGroupName,
				GangLevelLabelKey:                       string(workloadv1alpha1.GangScheduleLevelGroup),
			},
			Annotations: map[string]string{
				VolcanoGroupNameAnnotation: podGroupName,
				CreatedByAnnotation:        CreatedByValue,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: mi.APIVersion,
					Kind:       mi.Kind,
					Name:       mi.Name,
					UID:        mi.UID,
					Controller: &[]bool{true}[0],
				},
			},
		},
		Spec: schedulingv1beta1.PodGroupSpec{
			MinMember:     int32(minMember),
			MinTaskMember: minTaskMember,
			MinResources:  &minResources,
		},
	}

	_, err := m.volcanoClient.SchedulingV1beta1().PodGroups(mi.Namespace).Create(ctx, podGroup, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	klog.V(2).Infof("Created PodGroup %s for group-level gang scheduling", podGroupName)
	return nil
}

// createRoleLevelPodGroup creates a PodGroup for role-level gang scheduling
func (m *Manager) createRoleLevelPodGroup(ctx context.Context, mi *workloadv1alpha1.ModelInfer, inferGroupIndex int) error {
	podGroupName := m.generateGroupLevelPodGroupName(mi.Name, inferGroupIndex)

	// Calculate requirements based on MinRoleReplicas
	minMember, minTaskMember, minResources := m.calculateRoleLevelRequirements(mi)

	podGroup := &schedulingv1beta1.PodGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podGroupName,
			Namespace: mi.Namespace,
			Labels: map[string]string{
				workloadv1alpha1.ModelInferNameLabelKey: mi.Name,
				workloadv1alpha1.GroupNameLabelKey:      podGroupName,
				GangLevelLabelKey:                       string(workloadv1alpha1.GangScheduleLevelRole),
			},
			Annotations: map[string]string{
				VolcanoGroupNameAnnotation: podGroupName,
				CreatedByAnnotation:        CreatedByValue,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: mi.APIVersion,
					Kind:       mi.Kind,
					Name:       mi.Name,
					UID:        mi.UID,
					Controller: &[]bool{true}[0],
				},
			},
		},
		Spec: schedulingv1beta1.PodGroupSpec{
			MinMember:     int32(minMember),
			MinTaskMember: minTaskMember,
			MinResources:  &minResources,
		},
	}

	_, err := m.volcanoClient.SchedulingV1beta1().PodGroups(mi.Namespace).Create(ctx, podGroup, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	klog.V(2).Infof("Created PodGroup %s for role-level gang scheduling", podGroupName)
	return nil
}

// calculateGroupLevelRequirements calculates requirements for group-level gang scheduling
func (m *Manager) calculateGroupLevelRequirements(mi *workloadv1alpha1.ModelInfer) (int, map[string]int32, corev1.ResourceList) {
	minMember := 0
	minTaskMember := make(map[string]int32)
	minResources := corev1.ResourceList{}

	// For group-level, include all role instances
	for _, role := range mi.Spec.Template.Roles {
		roleReplicas := int(*role.Replicas)
		for roleIndex := 0; roleIndex < roleReplicas; roleIndex++ {
			taskName := m.generateTaskName(role.Name, roleIndex)
			podsPerTask := 1 + int(role.WorkerReplicas) // entry + workers
			minTaskMember[taskName] = int32(podsPerTask)
			minMember += podsPerTask

			// Aggregate resources
			m.aggregateResources(&minResources, &role.EntryTemplate.Spec)
			if role.WorkerTemplate != nil {
				for i := 0; i < int(role.WorkerReplicas); i++ {
					m.aggregateResources(&minResources, &role.WorkerTemplate.Spec)
				}
			}
		}
	}

	return minMember, minTaskMember, minResources
}

// calculateRoleLevelRequirements calculates requirements for role-level gang scheduling
func (m *Manager) calculateRoleLevelRequirements(mi *workloadv1alpha1.ModelInfer) (int, map[string]int32, corev1.ResourceList) {
	minMember := 0
	minTaskMember := make(map[string]int32)
	minResources := corev1.ResourceList{}

	// For role-level, only include roles up to MinRoleReplicas limit
	for _, role := range mi.Spec.Template.Roles {
		roleReplicas := int(*role.Replicas)
		minRoleReplicas := roleReplicas // Default to all replicas

		if mi.Spec.Template.GangSchedule.MinRoleReplicas != nil {
			if minReplicas, exists := mi.Spec.Template.GangSchedule.MinRoleReplicas[role.Name]; exists {
				minRoleReplicas = int(minReplicas)
			}
		}

		// Only include role replicas up to the minimum required
		for roleIndex := 0; roleIndex < minRoleReplicas && roleIndex < roleReplicas; roleIndex++ {
			taskName := m.generateTaskName(role.Name, roleIndex)
			podsPerTask := 1 + int(role.WorkerReplicas) // entry + workers
			minTaskMember[taskName] = int32(podsPerTask)
			minMember += podsPerTask

			// Aggregate resources
			m.aggregateResources(&minResources, &role.EntryTemplate.Spec)
			if role.WorkerTemplate != nil {
				for i := 0; i < int(role.WorkerReplicas); i++ {
					m.aggregateResources(&minResources, &role.WorkerTemplate.Spec)
				}
			}
		}
	}

	return minMember, minTaskMember, minResources
}

// aggregateResources aggregates resource requirements from a pod spec
func (m *Manager) aggregateResources(total *corev1.ResourceList, podSpec *corev1.PodSpec) {
	if *total == nil {
		*total = corev1.ResourceList{}
	}

	for _, container := range podSpec.Containers {
		for resourceName, quantity := range container.Resources.Requests {
			if existing, exists := (*total)[resourceName]; exists {
				existing.Add(quantity)
				(*total)[resourceName] = existing
			} else {
				(*total)[resourceName] = quantity.DeepCopy()
			}
		}
	}
}

// generateGroupLevelPodGroupName generates PodGroup name for group-level scheduling
func (m *Manager) generateGroupLevelPodGroupName(modelInferName string, inferGroupIndex int) string {
	return fmt.Sprintf("%s-%d", modelInferName, inferGroupIndex)
}

// generateTaskName generates task name for MinTaskMember
func (m *Manager) generateTaskName(roleName string, roleIndex int) string {
	return fmt.Sprintf("%s-%d", roleName, roleIndex)
}

// getExistingPodGroups gets existing PodGroups for a ModelInfer
func (m *Manager) getExistingPodGroups(ctx context.Context, mi *workloadv1alpha1.ModelInfer) (map[string]*schedulingv1beta1.PodGroup, error) {
	selector := labels.SelectorFromSet(map[string]string{
		workloadv1alpha1.ModelInferNameLabelKey: mi.Name,
	})

	podGroupList, err := m.volcanoClient.SchedulingV1beta1().PodGroups(mi.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}

	result := make(map[string]*schedulingv1beta1.PodGroup)
	for i := range podGroupList.Items {
		pg := &podGroupList.Items[i]
		result[pg.Name] = pg
	}

	return result, nil
}

// updatePodGroupIfNeeded updates a PodGroup if needed for group-level scheduling
func (m *Manager) updatePodGroupIfNeeded(ctx context.Context, existing *schedulingv1beta1.PodGroup, mi *workloadv1alpha1.ModelInfer, inferGroupIndex int) error {
	// Calculate current requirements
	minMember, minTaskMember, minResources := m.calculateGroupLevelRequirements(mi)

	needsUpdate := false
	updated := existing.DeepCopy()

	// Check if MinMember needs update
	if updated.Spec.MinMember != int32(minMember) {
		updated.Spec.MinMember = int32(minMember)
		needsUpdate = true
	}

	// Check if MinTaskMember needs update
	if !m.equalMinTaskMember(updated.Spec.MinTaskMember, minTaskMember) {
		updated.Spec.MinTaskMember = minTaskMember
		needsUpdate = true
	}

	// Check if MinResources needs update
	if !m.equalResourceList(updated.Spec.MinResources, &minResources) {
		updated.Spec.MinResources = &minResources
		needsUpdate = true
	}

	if needsUpdate {
		_, err := m.volcanoClient.SchedulingV1beta1().PodGroups(mi.Namespace).Update(ctx, updated, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		klog.V(2).Infof("Updated PodGroup %s for group-level gang scheduling", existing.Name)
	}

	return nil
}

// updateRoleLevelPodGroupIfNeeded updates a PodGroup if needed for role-level scheduling
func (m *Manager) updateRoleLevelPodGroupIfNeeded(ctx context.Context, existing *schedulingv1beta1.PodGroup, mi *workloadv1alpha1.ModelInfer, inferGroupIndex int) error {
	// Calculate current requirements
	minMember, minTaskMember, minResources := m.calculateRoleLevelRequirements(mi)

	needsUpdate := false
	updated := existing.DeepCopy()

	// Check if MinMember needs update
	if updated.Spec.MinMember != int32(minMember) {
		updated.Spec.MinMember = int32(minMember)
		needsUpdate = true
	}

	// Check if MinTaskMember needs update
	if !m.equalMinTaskMember(updated.Spec.MinTaskMember, minTaskMember) {
		updated.Spec.MinTaskMember = minTaskMember
		needsUpdate = true
	}

	// Check if MinResources needs update
	if !m.equalResourceList(updated.Spec.MinResources, &minResources) {
		updated.Spec.MinResources = &minResources
		needsUpdate = true
	}

	if needsUpdate {
		_, err := m.volcanoClient.SchedulingV1beta1().PodGroups(mi.Namespace).Update(ctx, updated, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		klog.V(2).Infof("Updated PodGroup %s for role-level gang scheduling", existing.Name)
	}

	return nil
}

// cleanupExcessPodGroups cleans up excess PodGroups
func (m *Manager) cleanupExcessPodGroups(ctx context.Context, mi *workloadv1alpha1.ModelInfer, existingPodGroups map[string]*schedulingv1beta1.PodGroup, expectedReplicas int) error {
	for podGroupName, podGroup := range existingPodGroups {
		// Check if this PodGroup is still needed
		isNeeded := false
		for i := 0; i < expectedReplicas; i++ {
			expectedName := m.generateGroupLevelPodGroupName(mi.Name, i)
			if podGroupName == expectedName {
				isNeeded = true
				break
			}
		}

		if !isNeeded {
			err := m.volcanoClient.SchedulingV1beta1().PodGroups(mi.Namespace).Delete(ctx, podGroup.Name, metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete excess PodGroup %s: %v", podGroup.Name, err)
			}
			klog.V(2).Infof("Deleted excess PodGroup %s", podGroup.Name)
		}
	}

	return nil
}

// cleanupPodGroups cleans up all PodGroups for a ModelInfer
func (m *Manager) cleanupPodGroups(ctx context.Context, mi *workloadv1alpha1.ModelInfer) error {
	existingPodGroups, err := m.getExistingPodGroups(ctx, mi)
	if err != nil {
		return fmt.Errorf("failed to get existing PodGroups for cleanup: %v", err)
	}

	for _, podGroup := range existingPodGroups {
		err := m.volcanoClient.SchedulingV1beta1().PodGroups(mi.Namespace).Delete(ctx, podGroup.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete PodGroup %s: %v", podGroup.Name, err)
		}
		klog.V(2).Infof("Deleted PodGroup %s (gang scheduling disabled)", podGroup.Name)
	}

	return nil
}

// AnnotatePodWithPodGroup annotates a pod with the appropriate PodGroup information
func (m *Manager) AnnotatePodWithPodGroup(pod *corev1.Pod, mi *workloadv1alpha1.ModelInfer, inferGroupIndex int) {
	if !m.isGangSchedulingEnabled(mi) {
		return
	}

	podGroupName := m.generateGroupLevelPodGroupName(mi.Name, inferGroupIndex)

	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}

	pod.Annotations[VolcanoGroupNameAnnotation] = podGroupName
}

// equalMinTaskMember compares two MinTaskMember maps
func (m *Manager) equalMinTaskMember(a, b map[string]int32) bool {
	if len(a) != len(b) {
		return false
	}

	for key, valueA := range a {
		if valueB, exists := b[key]; !exists || valueA != valueB {
			return false
		}
	}

	return true
}

// equalResourceList compares two ResourceList
func (m *Manager) equalResourceList(a, b *corev1.ResourceList) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	aList := *a
	bList := *b

	if len(aList) != len(bList) {
		return false
	}

	for resourceName, quantityA := range aList {
		if quantityB, exists := bList[resourceName]; !exists || !quantityA.Equal(quantityB) {
			return false
		}
	}

	return true
}
