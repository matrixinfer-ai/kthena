/*
Copyright 2024.

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
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
)

// ModelRouteController reconciles a ModelRoute object
type ModelRouteController struct {
	client.Client
	Scheme *runtime.Scheme

	ResourceToModels map[string]string
	ResourceToLoras  map[string][]string
	Routes           map[string]*aiv1alpha1.ModelRoute
	LoraRoutes       map[string]*aiv1alpha1.ModelRoute
}

func NewModelRouteController(mgr ctrl.Manager) *ModelRouteController {
	return &ModelRouteController{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),

		ResourceToModels: make(map[string]string),
		ResourceToLoras:  make(map[string][]string),
		Routes:           make(map[string]*aiv1alpha1.ModelRoute),
		LoraRoutes:       make(map[string]*aiv1alpha1.ModelRoute),
	}
}

// +kubebuilder:rbac:groups=ai.kmesh.net,resources=ModelRoutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ai.kmesh.net,resources=ModelRoutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ai.kmesh.net,resources=ModelRoutes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ModelRoute object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/reconcile
func (m *ModelRouteController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	var mr aiv1alpha1.ModelRoute
	if err := m.Get(ctx, req.NamespacedName, &mr); err != nil {
		if apierrors.IsNotFound(err) {
			model, loras := m.GetFromResourceMap(req.NamespacedName.String())
			if err := m.DeleteRoute(model, loras); err != nil {
				log.Errorf("failed to delete route: %v", err)
				return ctrl.Result{}, nil
			}
		}

		log.Errorf("unable to fetch ModelRoute: %v", err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Infof("Get ModelRoute model: %s", mr.Spec.ModelName)

	m.SetForResourceMap(req.NamespacedName.String(), mr.Spec.ModelName, mr.Spec.LoraAdapters)

	// TODO: make sure only one modelRoute take effect?
	if err := m.UpdateRoute(mr.Spec.ModelName, mr.Spec.LoraAdapters, &mr); err != nil {
		log.Error(err, "failed to update ModelRouter")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (m *ModelRouteController) GetFromResourceMap(namespacedName string) (string, []string) {
	return m.ResourceToModels[namespacedName], m.ResourceToLoras[namespacedName]
}

func (m *ModelRouteController) SetForResourceMap(namespacedName string, model string, loras []string) {
	m.ResourceToModels[namespacedName] = model
	m.ResourceToLoras[namespacedName] = loras
}

// Match matches the model name and http request, and returns the model server name and whether it is a lora model
func (m *ModelRouteController) Match(model string, req *http.Request) (types.NamespacedName, bool, error) {
	var is_lora bool
	mr, ok := m.Routes[model]
	if !ok {
		mr, ok = m.LoraRoutes[model]
		if !ok {
			return types.NamespacedName{}, false, fmt.Errorf("not found route rules for model %s", model)
		}
		is_lora = true
	}

	rule, err := m.selectRule(req, mr.Spec.Rules)
	if err != nil {
		return types.NamespacedName{}, false, fmt.Errorf("failed to select route rule: %v", err)
	}

	dst, err := m.selectDestination(rule.TargetModels)
	if err != nil {
		return types.NamespacedName{}, false, fmt.Errorf("failed to select destination: %v", err)
	}

	return types.NamespacedName{Namespace: mr.Namespace, Name: dst.ModelServerName}, is_lora, nil
}

func (m *ModelRouteController) selectRule(req *http.Request, rules []*aiv1alpha1.Rule) (*aiv1alpha1.Rule, error) {
	for _, rule := range rules {
		if rule.ModelMatch == nil {
			return rule, nil
		}

		headersMatched := true
		for key, sm := range rule.ModelMatch.Headers {
			reqValue := req.Header.Get(key)
			if !matchString(sm, reqValue) {
				headersMatched = false
				break
			}
		}
		if !headersMatched {
			continue
		}

		uriMatched := true
		if uriMatch := rule.ModelMatch.Uri; uriMatch != nil {
			if !matchString(uriMatch, req.URL.Path) {
				uriMatched = false
			}
		}

		if !uriMatched {
			continue
		}

		return rule, nil
	}

	return nil, fmt.Errorf("failed to find a matching rule")
}

func matchString(sm *aiv1alpha1.StringMatch, value string) bool {
	switch {
	case sm.Exact != nil:
		return value == *sm.Exact
	case sm.Prefix != nil:
		return strings.HasPrefix(value, *sm.Prefix)
	case sm.Regex != nil:
		matched, _ := regexp.MatchString(*sm.Regex, value)
		return matched
	default:
		return true
	}
}

func (m *ModelRouteController) selectDestination(targets []*aiv1alpha1.TargetModel) (*aiv1alpha1.TargetModel, error) {
	weightedSlice, err := toWeightedSlice(targets)
	if err != nil {
		return nil, err
	}

	index := selectFromWeightedSlice(weightedSlice)

	return targets[index], nil
}

func toWeightedSlice(targets []*aiv1alpha1.TargetModel) ([]uint32, error) {
	var isWeighted bool
	if targets[0].Weight != nil {
		isWeighted = true
	}

	res := make([]uint32, len(targets))

	for i, target := range targets {
		if (isWeighted && target.Weight == nil) || (!isWeighted && target.Weight != nil) {
			return nil, fmt.Errorf("the weight field in targetModel must be either fully specified or not specified")
		}

		if isWeighted {
			res[i] = *target.Weight
		} else {
			// If weight is not specified, set to 1.
			res[i] = 1
		}
	}

	return res, nil
}

func selectFromWeightedSlice(weights []uint32) int {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	totalWeight := 0
	for _, weight := range weights {
		totalWeight += int(weight)
	}

	randomNum := rng.Intn(totalWeight)

	for i, weight := range weights {
		randomNum -= int(weight)
		if randomNum < 0 {
			return i
		}
	}

	return 0
}

func (m *ModelRouteController) UpdateRoute(model string, loras []string, mr *aiv1alpha1.ModelRoute) error {
	log.Infof("UpdateRoute model: %v, loras: %v", model, loras)

	if model != "" {
		m.Routes[model] = mr
	}

	for _, lora := range loras {
		m.LoraRoutes[lora] = mr
	}

	return nil
}

func (m *ModelRouteController) DeleteRoute(model string, loras []string) error {
	log.Infof("DeleteRoute model: %v, loras: %v", model, loras)

	delete(m.Routes, model)

	for _, lora := range loras {
		delete(m.LoraRoutes, lora)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (m *ModelRouteController) SetupWithManager(mgr ctrl.Manager) error {
	log.Infof("start modelroutes controller")

	return ctrl.NewControllerManagedBy(mgr).
		For(&aiv1alpha1.ModelRoute{}).
		Named("ModelRoute").
		Complete(m)
}
