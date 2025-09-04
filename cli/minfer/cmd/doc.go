/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// docCmd represents the doc command
var docCmd = &cobra.Command{
	Use:   "doc",
	Short: "Generate documentation for the minfer CLI",
	Long: `Generate documentation for the minfer CLI in various formats.

This command uses Cobra's built-in documentation generation to create
comprehensive documentation for all commands and subcommands.

Examples:
  minfer doc --output ./docs --format markdown
  minfer doc --output ./docs --format man
  minfer doc --output ./docs --format yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		outputDir, _ := cmd.Flags().GetString("output")
		format, _ := cmd.Flags().GetString("format")
		
		// Create output directory if it doesn't exist
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		fmt.Printf("Generating %s documentation in %s...\n", format, outputDir)

		switch format {
		case "markdown", "md":
			err := doc.GenMarkdownTree(rootCmd, outputDir)
			if err != nil {
				return fmt.Errorf("failed to generate markdown documentation: %w", err)
			}
		case "man":
			header := &doc.GenManHeader{
				Title:   "MINFER",
				Section: "1",
				Source:  "MatrixInfer CLI",
			}
			err := doc.GenManTree(rootCmd, header, outputDir)
			if err != nil {
				return fmt.Errorf("failed to generate man pages: %w", err)
			}
		case "yaml":
			err := doc.GenYamlTree(rootCmd, outputDir)
			if err != nil {
				return fmt.Errorf("failed to generate YAML documentation: %w", err)
			}
		case "rest":
			err := doc.GenReSTTree(rootCmd, outputDir)
			if err != nil {
				return fmt.Errorf("failed to generate ReStructuredText documentation: %w", err)
			}
		default:
			return fmt.Errorf("unsupported format: %s. Supported formats: markdown, man, yaml, rest", format)
		}

		fmt.Printf("Documentation generated successfully in %s\n", outputDir)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(docCmd)

	// Define flags
	docCmd.Flags().StringP("output", "o", "./docs", "Output directory for generated documentation")
	docCmd.Flags().StringP("format", "f", "markdown", "Output format (markdown, man, yaml, rest)")
}