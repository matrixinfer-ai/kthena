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

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	"github.com/volcano-sh/kthena/cmd/infer-router/app"
)

func main() {
	var (
		port    string
		tlsCert string
		tlsKey  string
	)

	// Initialize klog flags
	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.StringVar(&port, "port", "8080", "Server listen port")
	pflag.StringVar(&tlsCert, "tls-cert", "", "TLS certificate file path")
	pflag.StringVar(&tlsKey, "tls-key", "", "TLS key file path")
	defer klog.Flush()
	pflag.Parse()

	if (tlsCert != "" && tlsKey == "") || (tlsCert == "" && tlsKey != "") {
		klog.Fatal("tls-cert and tls-key must be specified together")
	}

	pflag.CommandLine.VisitAll(func(f *pflag.Flag) {
		// print all flags for debugging
		klog.Infof("Flag: %s, Value: %s", f.Name, f.Value.String())
	})

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	// Wait for a signal
	go func() {
		<-signalCh
		klog.Info("Received termination, signaling shutdown")
		cancel()
	}()
	app.NewServer(port, tlsCert != "" && tlsKey != "", tlsCert, tlsKey).Run(ctx)
}
