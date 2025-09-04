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
