/*
Copyright The Volcano Authors.

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

package controller

import (
	"context"

	networking "github.com/volcano-sh/kthena/pkg/apis/networking/v1alpha1"
	workloadv1alpha1 "github.com/volcano-sh/kthena/pkg/apis/workload/v1alpha1"
	"github.com/volcano-sh/kthena/pkg/model-booster-controller/convert"
	"github.com/volcano-sh/kthena/pkg/model-booster-controller/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
)

func (mc *ModelController) createOrUpdateModelServer(ctx context.Context, model *workloadv1alpha1.ModelBooster) error {
	existingModelServers, err := mc.listModelServerByLabel(model)
	if err != nil {
		return err
	}
	modelServers, err := convert.BuildModelServer(model)
	if err != nil {
		return err
	}
	modelServersToKeep := make(map[string]struct{})
	for _, modelServer := range modelServers {
		modelServersToKeep[modelServer.Name] = struct{}{}
		oldModelServer, err := mc.modelServersLister.ModelServers(modelServer.Namespace).Get(modelServer.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				klog.V(4).Infof("Create ModelBooster Server %s", modelServer.Name)
				if _, err := mc.client.NetworkingV1alpha1().ModelServers(model.Namespace).Create(ctx, modelServer, metav1.CreateOptions{}); err != nil {
					klog.Errorf("failed to create ModelServer %s: %v", klog.KObj(modelServer), err)
					return err
				}
				continue
			}
			klog.Errorf("failed to get ModelServer %s: %v", klog.KObj(modelServer), err)
			return err
		}
		if oldModelServer.Labels[utils.RevisionLabelKey] == modelServer.Labels[utils.RevisionLabelKey] {
			klog.Infof("ModelBooster Server %s of model %s does not need to update", modelServer.Name, model.Name)
			continue
		}
		modelServer.ResourceVersion = oldModelServer.ResourceVersion
		if _, err := mc.client.NetworkingV1alpha1().ModelServers(model.Namespace).Update(ctx, modelServer, metav1.UpdateOptions{}); err != nil {
			klog.Errorf("failed to update ModelServer %s: %v", klog.KObj(modelServer), err)
			return err
		}
		klog.V(4).Infof("Updated ModelBooster Server %s for model %s", modelServer.Name, model.Name)
	}
	for _, existingModelServer := range existingModelServers {
		if _, ok := modelServersToKeep[existingModelServer.Name]; !ok {
			if err := mc.client.NetworkingV1alpha1().ModelServers(model.Namespace).Delete(ctx, existingModelServer.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			klog.V(4).Infof("Delete ModelServer %s", existingModelServer.Name)
		}
	}
	return nil
}

func (mc *ModelController) listModelServerByLabel(model *workloadv1alpha1.ModelBooster) ([]*networking.ModelServer, error) {
	if modelServers, err := mc.modelServersLister.ModelServers(model.Namespace).List(labels.SelectorFromSet(map[string]string{
		utils.OwnerUIDKey: string(model.UID),
	})); err != nil {
		return nil, err
	} else {
		return modelServers, nil
	}
}
