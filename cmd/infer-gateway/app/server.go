package app

import (
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/controller"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

type Server struct {
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Run(stop <-chan struct{}) {
	// create store
	store := datastore.New()

	// Start store's periodic update loop
	go store.Run(stop)

	go func() {
		// start controller
		if err := controller.StartControllers(store); err != nil {
			log.Fatal("Unable to start controllers")
		}
	}()

	// start router
	startRouter(stop, store)
}
