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
	"fmt"
	"log"
	"os"

	"matrixinfer.ai/matrixinfer/cli/minfer/cmd"

	"github.com/spf13/cobra/doc"
)

func main() {
	// Get the root command from the CLI application
	rootCmd := cmd.GetRootCmd()

	// Define output directory relative to the project root (assumes running from repo root)
	// Target: docs/matrixinfer/docs/reference/cli
	outputDir := "docs/matrixinfer/docs/reference/cli"

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		log.Fatalf("Error creating doc output directory: %v", err)
	}

	// Generate Markdown documentation
	if err := doc.GenMarkdownTree(rootCmd, outputDir); err != nil {
		log.Fatalf("Error generating Markdown documentation: %v", err)
	}
	fmt.Printf("Markdown documentation generated in %s\n", outputDir)
}
