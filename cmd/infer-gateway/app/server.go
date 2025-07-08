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

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/controller"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

type Server struct {
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Run(ctx context.Context) {
	// create store
	store := datastore.New()

	r := NewRouter(store)

	// Start store's periodic update loop
	go store.Run(ctx)

	go func() {
		// start controller
		// must be after the store
		if err := controller.StartControllers(store); err != nil {
			log.Fatal("Unable to start controllers")
		}
	}()
	// start router
	startRouter(ctx, r)
}
