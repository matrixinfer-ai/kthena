// List-watch modelRoute and modelServer
package controller

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	v1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/logger"
)

var (
	log = logger.NewLogger("controller")
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.Install(scheme))
}

func StartControllers(store datastore.Store) error {
	// start controller
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			// disable metrics of controller now.
			BindAddress: "0",
		},
	})
	if err != nil {
		log.Errorf("Unable to start manager")
		return err
	}

	mrc := NewModelRouteController(mgr, store)
	if err := mrc.SetupWithManager(mgr); err != nil {
		log.Errorf("Unable to start Model Route Controller: %v", err)
		return err
	}

	msc := NewModelServerController(mgr, store)
	if err := msc.SetupWithManager(mgr); err != nil {
		log.Errorf("Unable to start Model Server Controller: %v", err)
		return err
	}

	pc := NewPodController(mgr, store)
	if err := pc.SetupWithManager(mgr); err != nil {
		log.Errorf("Unable to start Pod Controller: %v", err)
		return err
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Errorf("Unable to start manager: %v", err)
		return err
	}

	return nil
}
