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

	workloadv1alpha1 "github.com/volcano-sh/kthena/pkg/apis/workload/v1alpha1"
	"github.com/volcano-sh/kthena/pkg/model-booster-controller/convert"
	"github.com/volcano-sh/kthena/pkg/model-booster-controller/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (mc *ModelController) createOrUpdateModelRoute(ctx context.Context, model *workloadv1alpha1.ModelBooster) error {
	modelRoute := convert.BuildModelRoute(model)
	oldModelRoute, err := mc.modelRoutesLister.ModelRoutes(modelRoute.Namespace).Get(modelRoute.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("Create ModelBooster Route %s", modelRoute.Name)
			if _, err := mc.client.NetworkingV1alpha1().ModelRoutes(model.Namespace).Create(ctx, modelRoute, metav1.CreateOptions{}); err != nil {
				klog.Errorf("failed to create ModelRoute %s: %v", klog.KObj(modelRoute), err)
				return err
			}
			return nil
		}
		klog.Errorf("failed to get ModelRoute %s: %v", klog.KObj(modelRoute), err)
		return err
	}
	if oldModelRoute.Labels[utils.RevisionLabelKey] == modelRoute.Labels[utils.RevisionLabelKey] {
		klog.Infof("ModelBooster Route %s of model %s does not need to update", modelRoute.Name, model.Name)
		return nil
	}
	modelRoute.ResourceVersion = oldModelRoute.ResourceVersion
	if _, err := mc.client.NetworkingV1alpha1().ModelRoutes(model.Namespace).Update(ctx, modelRoute, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("failed to update ModelRoute %s: %v", klog.KObj(modelRoute), err)
		return err
	}
	return nil
}
