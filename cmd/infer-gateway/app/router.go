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

var log = logger.NewLogger("")

// Starts router
func startRouter(stop <-chan struct{}, store datastore.Store) {
	engine := gin.New()
	engine.Use(gin.LoggerWithWriter(gin.DefaultWriter, "/healthz"), gin.Recovery())

	// TODO: add middle ware
	// engine.Use()

	// engine.Use(auth.Authenticate)
	// engine.Use(auth.Authorize)

	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "ok",
		})
	})

	r := router.NewRouter(store)

	// Handle all paths under /v1/
	engine.Any("/v1/*path", r.HandlerFunc())

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

	<-stop

	// graceful shutdown
	log.Info("Shutting down HTTP server ...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Info("Server Shutdown:", err)
	}
	// catching ctx.Done(). timeout of 5 seconds.
	<-ctx.Done()
	log.Info("timeout of 5 seconds.")
	log.Info("HTTP server exiting")
}
