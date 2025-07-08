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

package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"matrixinfer.ai/matrixinfer/cmd/infer-gateway/app"
)

const gracefulShutdownTimeout = 15 * time.Second

func main() {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	stopCh := make(chan struct{})
	app.NewServer().Run(stopCh)

	// Wait for a signal
	<-signalCh
	close(stopCh)

	// Give some timne for the gateway server to finish processing requests
	time.Sleep(gracefulShutdownTimeout)
}
