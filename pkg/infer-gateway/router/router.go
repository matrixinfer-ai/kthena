package router

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	aiv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/networking/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/controller"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/logger"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/scheduler"
)

var (
	scheme = runtime.NewScheme()
	log    = logger.NewLogger("router")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(aiv1alpha1.Install(scheme))
	// Initialize a logger for the controller runtime.
	ctrllog.SetLogger(klog.NewKlogr())
	// +kubebuilder:scaffold:scheme
}

type Router struct {
	// Define the fields of the Router struct here
	modelRouteController  *controller.ModelRouteController
	modelServerController *controller.ModelServerController
	podController         *controller.PodController

	scheduler scheduler.Scheduler
	store     datastore.Store
}

var _ gin.HandlerFunc

func NewRouter() *Router {
	return &Router{}
}

func (r *Router) Run(stop <-chan struct{}) {
	log.Infof("start router")

	store, err := datastore.New()
	if err != nil {
		log.Errorf("Unable to create data store")
		os.Exit(1)
	}
	r.store = store

	r.scheduler = scheduler.NewScheduler(r.store)

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
		os.Exit(1)
	}

	mrc := controller.NewModelRouteController(mgr)
	if mrc.SetupWithManager(mgr); err != nil {
		log.Errorf("Unable to start Model Route Controller: %v", err)
		os.Exit(1)
	}
	r.modelRouteController = mrc

	msc := controller.NewModelServerController(mgr, r.store)
	if err := msc.SetupWithManager(mgr); err != nil {
		log.Errorf("Unable to start Model Server Controller: %v", err)
		os.Exit(1)
	}
	r.modelServerController = msc

	pc := controller.NewPodController(mgr, r.store)
	if err := pc.SetupWithManager(mgr); err != nil {
		log.Errorf("Unable to start Pod Controller: %v", err)
		os.Exit(1)
	}
	r.podController = pc

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Errorf("Unable to start manager: %v", err)
	}

}

type ModelRequest map[string]interface{}

func (r *Router) HandlerFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		// implement gin request body reading here
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, err)
			return
		}
		var modelRequest ModelRequest
		if err := json.Unmarshal(bodyBytes, &modelRequest); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, err)
			return
		}

		modelName, ok := modelRequest["model"].(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusNotFound, "model not found")
			return
		}

		log.Debugf("model name is %v", modelName)

		// find the corresponding model server bt matching request and model name.
		modelServerName, is_lora, err := r.modelRouteController.Match(modelName, c.Request)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, fmt.Sprintf("can't find corresponding model server: %v", err))
			return
		}

		log.Debugf("modelServer is %v, is_lora: %v", modelServerName, is_lora)

		// according to modelRequest.Model, route to different model
		// call scheduler to select a model
		pods, model, err := r.modelServerController.GetEndpoints(modelServerName)
		if err != nil || len(pods) == 0 {
			c.AbortWithStatusJSON(http.StatusNotFound, fmt.Sprintf("can't find target pods of model server: %v, err: %v", modelServerName, err))
			return
		}
		// Overwrite model.
		if model != nil && !is_lora {
			modelRequest["model"] = *model
		}

		// call scheduler.Schedule
		targetPod, err := r.scheduler.Schedule(modelRequest, pods)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, fmt.Sprintf("can't schedule to target pod: %v", err))
			return
		}

		req := c.Request

		original := req.Host
		log.Infof("host is %s, scheme is %s", original, req.URL.Scheme)
		_, port, err := net.SplitHostPort(original)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, fmt.Sprintf("failed to split host and port from url host: %v", err))
			return
		}

		body, err := json.Marshal(modelRequest)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, fmt.Sprintf("marshal http body failed: %v", err))
			return
		}

		// step 1: change request URL to real server URL.
		req.URL.Host = fmt.Sprintf("%s:%s", targetPod.Pod.Status.PodIP, port)
		req.URL.Scheme = "http"
		req.Body = io.NopCloser(bytes.NewBuffer(body))
		req.ContentLength = int64(len(body))

		// step 2: use http.Transport to do request to real server.
		transport := http.DefaultTransport
		resp, err := transport.RoundTrip(req)
		if err != nil {
			log.Errorf("error: %v", err)
			c.String(500, "error")
			return
		}

		// step 3: return real server response to downstream.
		for k, vv := range resp.Header {
			for _, v := range vv {
				c.Header(k, v)
			}
		}
		defer resp.Body.Close()
		// Maybe we need to read the response to get the tokens for ratelimiting later
		bufio.NewReader(resp.Body).WriteTo(c.Writer)
	}
}
