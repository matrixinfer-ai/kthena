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
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/volcano-sh/kthena/pkg/infer-gateway/datastore"
	"github.com/volcano-sh/kthena/pkg/infer-gateway/debug"
	"github.com/volcano-sh/kthena/pkg/infer-gateway/router"
)

const (
	gracefulShutdownTimeout = 15 * time.Second
	gatewayConfigFile       = "/etc/config/gatewayConfiguration.yaml"
)

func NewRouter(store datastore.Store) *router.Router {
	return router.NewRouter(store, gatewayConfigFile)
}

// Starts router
func (s *Server) startRouter(ctx context.Context, router *router.Router, store datastore.Store) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.LoggerWithWriter(gin.DefaultWriter, "/healthz", "readyz"), gin.Recovery())

	// TODO: add middle ware
	// engine.Use()

	// engine.Use(auth.Authenticate)
	// engine.Use(auth.Authorize)
	engine.Use(AccessLogMiddleware(router))
	engine.Use(AuthMiddleware(router))

	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "ok",
		})
	})

	engine.GET("/readyz", func(c *gin.Context) {
		if s.HasSynced() {
			c.JSON(http.StatusOK, gin.H{
				"message": "gateway is ready",
			})
		} else {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"message": "gateway is not ready",
			})
		}
	})

	// Handle all paths under /v1/
	engine.Any("/v1/*path", router.HandlerFunc())

	// Debug endpoints
	debugHandler := debug.NewDebugHandler(store)
	debugGroup := engine.Group("/debug/config_dump")
	{
		// List resources
		debugGroup.GET("/modelroutes", debugHandler.ListModelRoutes)
		debugGroup.GET("/modelservers", debugHandler.ListModelServers)
		debugGroup.GET("/pods", debugHandler.ListPods)

		// Get specific resources
		debugGroup.GET("/namespaces/:namespace/modelroutes/:name", debugHandler.GetModelRoute)
		debugGroup.GET("/namespaces/:namespace/modelservers/:name", debugHandler.GetModelServer)
		debugGroup.GET("/namespaces/:namespace/pods/:name", debugHandler.GetPod)
	}

	server := &http.Server{
		Addr:    ":" + s.Port,
		Handler: engine.Handler(),
	}
	go func() {
		// service connections
		var err error
		if s.EnableTLS {
			if s.TLSCertFile == "" || s.TLSKeyFile == "" {
				klog.Fatalf("TLS enabled but cert or key file not specified")
			}
			err = server.ListenAndServeTLS(s.TLSCertFile, s.TLSKeyFile)
		} else {
			err = server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			klog.Fatalf("listen failed: %v", err)
		}
	}()

	<-ctx.Done()
	// graceful shutdown
	klog.Info("Shutting down HTTP server ...")
	ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		klog.Errorf("Server shutdown failed: %v", err)
	}
	klog.Info("HTTP server exited")
}

func AccessLogMiddleware(gwRouter *router.Router) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Access log for "/v1/" only
		if !strings.HasPrefix(c.Request.URL.Path, "/v1/") {
			c.Next()
			return
		}

		// Calling Middleware
		gwRouter.AccessLog()(c)
	}
}

func AuthMiddleware(gwRouter *router.Router) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Auth for "/v1/" only
		if !strings.HasPrefix(c.Request.URL.Path, "/v1/") {
			c.Next()
			return
		}

		// Calling Middleware
		gwRouter.Auth()(c)
		if c.IsAborted() {
			return
		}

		c.Next()
	}
}
