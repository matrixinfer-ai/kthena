/*
Copyright © 2025 MatrixInfer-AI Authors

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
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// describeCmd represents the describe command
var describeCmd = &cobra.Command{
	Use:   "describe",
	Short: "Show detailed information about a specific resource",
	Long: `Show detailed information about a specific resource.

You can describe templates and other MatrixInfer resources.

Examples:
  minfer describe template deepseek-r1-distill-llama-8b`,
}

// describeTemplateCmd represents the describe template command
var describeTemplateCmd = &cobra.Command{
	Use:   "template [NAME]",
	Short: "Show detailed information about a template",
	Long: `Show detailed information about a specific template.

This will display the template description, available variables,
and the full template content.`,
	Args: cobra.ExactArgs(1),
	RunE: runDescribeTemplate,
}

func init() {
	rootCmd.AddCommand(describeCmd)
	describeCmd.AddCommand(describeTemplateCmd)
}

func runDescribeTemplate(cmd *cobra.Command, args []string) error {
	templateName := args[0]

	// Check if template exists
	if !TemplateExists(templateName) {
		return fmt.Errorf("template '%s' not found", templateName)
	}

	// Read template content from embedded files
	content, err := GetTemplateContent(templateName)
	if err != nil {
		return fmt.Errorf("failed to read template: %v", err)
	}

	fmt.Printf("Template: %s\n", templateName)
	fmt.Println("================")

	// Extract description
	description := extractManifestDescriptionFromContent(content)
	fmt.Printf("Description: %s\n\n", description)

	// Extract variables from template
	variables := extractTemplateVariables(content)
	if len(variables) > 0 {
		fmt.Println("Available Variables:")
		for _, variable := range variables {
			fmt.Printf("  - %s\n", variable)
		}
		fmt.Println()
	}

	fmt.Println("Template Content:")
	fmt.Println("=================")
	fmt.Println(content)

	return nil
}
