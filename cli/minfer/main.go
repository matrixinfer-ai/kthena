/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>

*/
package main

import (
	"embed"
	"matrixinfer.ai/matrixinfer/cli/minfer/cmd"
)

//go:embed templates/*.yaml
var templatesFS embed.FS

func main() {
	cmd.InitTemplates(templatesFS)
	cmd.Execute()
}
