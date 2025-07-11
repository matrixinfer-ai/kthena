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

package app

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/logger"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/router"
)

const gracefulShutdownTimeout = 15 * time.Second

var log = logger.NewLogger("")

func NewRouter(store datastore.Store) *router.Router {
	return router.NewRouter(store)
}

// Starts router
func startRouter(ctx context.Context, router *router.Router) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.LoggerWithWriter(gin.DefaultWriter, "/healthz"), gin.Recovery())

	// TODO: add middle ware
	// engine.Use()

	// engine.Use(auth.Authenticate)
	// engine.Use(auth.Authorize)

	// TODO: return healthy after the controller has been synced
	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "ok",
		})
	})

	// Handle all paths under /v1/
	engine.Any("/v1/*path", router.HandlerFunc())

	server := &http.Server{
		Addr:    ":8080",
		Handler: engine.Handler(),
	}
	go func() {
		// service connections
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen failed: %v", err)
		}
	}()

	<-ctx.Done()
	// graceful shutdown
	log.Info("Shutting down HTTP server ...")
	ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Info("Server Shutdown:", err)
	}
	log.Info("HTTP server exited")
}
