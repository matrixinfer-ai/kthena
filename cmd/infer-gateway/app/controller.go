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

package app

import (
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	clientset "github.com/volcano-sh/kthena/client-go/clientset/versioned"
	kthenaInformers "github.com/volcano-sh/kthena/client-go/informers/externalversions"
	"github.com/volcano-sh/kthena/pkg/infer-gateway/controller"
	"github.com/volcano-sh/kthena/pkg/infer-gateway/datastore"
)

type Controller interface {
	HasSynced() bool
}

type aggregatedController struct {
	controllers []Controller
}

var _ Controller = &aggregatedController{}

func startControllers(store datastore.Store, stop <-chan struct{}) Controller {
	cfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	kthenaClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kthena clientset: %s", err.Error())
	}

	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	kthenaInformerFactory := kthenaInformers.NewSharedInformerFactory(kthenaClient, 0)

	modelRouteController := controller.NewModelRouteController(kthenaInformerFactory, store)
	modelServerController := controller.NewModelServerController(kthenaInformerFactory, kubeInformerFactory, store)

	kubeInformerFactory.Start(stop)
	kthenaInformerFactory.Start(stop)

	go func() {
		if err := modelRouteController.Run(stop); err != nil {
			klog.Fatalf("Error running model route controller: %s", err.Error())
		}
	}()

	go func() {
		if err := modelServerController.Run(stop); err != nil {
			klog.Fatalf("Error running model server controller: %s", err.Error())
		}
	}()

	return &aggregatedController{
		controllers: []Controller{
			modelRouteController,
			modelServerController,
		},
	}
}

func (c *aggregatedController) HasSynced() bool {
	for _, controller := range c.controllers {
		if !controller.HasSynced() {
			return false
		}
	}
	return true
}
