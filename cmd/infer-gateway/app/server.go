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

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

type Server struct {
	store       datastore.Store
	controllers Controller
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Run(ctx context.Context) {
	// create store
	store := datastore.New()
	s.store = store

	// must be run before the controller, because it will register callbacks
	r := NewRouter(store)

	// Start store's periodic update loop
	go store.Run(ctx)

	// start controller
	s.controllers = startControllers(store, ctx.Done())
	// start router
	s.startRouter(ctx, r)
}

func (s *Server) HasSynced() bool {
	return s.controllers.HasSynced()
}
