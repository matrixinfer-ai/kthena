package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"

	"matrixinfer.ai/matrixinfer/cmd/infer-gateway/app"
)

var myFlag string

func main() {
	pflag.StringVar(&myFlag, "myFlag", "defaultValue", "Description of my flag")
	pflag.Parse()

	fmt.Println("myFlag:", myFlag)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	stopCh := make(chan struct{})
	app.NewServer().Run(stopCh)

	// Wait for a signal
	<-signalCh
	close(stopCh)
}
