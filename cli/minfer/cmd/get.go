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
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

var (
	outputFormat string
)

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Display one or many resources",
	Long: `Display one or many resources.

You can get templates, models, modelinfers, and autoscaling policies.

Examples:
  minfer get templates
  minfer get template deepseek-r1-distill-llama-8b
  minfer get template deepseek-r1-distill-llama-8b -o yaml
  minfer get models
  minfer get models --all-namespaces
  minfer get modelinfers -n production`,
}

// getTemplatesCmd represents the get templates command
var getTemplatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "List available manifest templates",
	Long: `List all available manifest templates that can be used with minfer commands.

Templates are predefined combinations of MatrixInfer resources that can be
customized with your specific values.`,
	RunE: runGetTemplates,
}

// getTemplateCmd represents the get template command
var getTemplateCmd = &cobra.Command{
	Use:   "template [NAME]",
	Short: "Get a specific template",
	Long: `Get a specific template by name.

Use -o yaml flag to output the template content in YAML format.`,
	Args: cobra.ExactArgs(1),
	RunE: runGetTemplate,
}

func init() {
	rootCmd.AddCommand(getCmd)
	getCmd.AddCommand(getTemplatesCmd)
	getCmd.AddCommand(getTemplateCmd)

	// Add output format flag
	getCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "", "Output format (yaml|json|table)")
}

func runGetTemplates(cmd *cobra.Command, args []string) error {
	templateNames, err := ListTemplates()
	if err != nil {
		return fmt.Errorf("failed to read templates: %v", err)
	}

	if len(templateNames) == 0 {
		fmt.Println("No templates found.")
		return nil
	}

	if outputFormat == "yaml" {
		var templates []ManifestInfo
		for _, templateName := range templateNames {
			manifestInfo, err := GetTemplateInfo(templateName)
			if err != nil {
				manifestInfo = ManifestInfo{
					Name:        templateName,
					Description: "No description available",
					FilePath:    fmt.Sprintf("%s.yaml", templateName),
				}
			}
			templates = append(templates, manifestInfo)
		}

		data, err := yaml.Marshal(templates)
		if err != nil {
			return fmt.Errorf("failed to marshal to YAML: %v", err)
		}
		fmt.Print(string(data))
		return nil
	}

	// Default table output
	var manifests []ManifestInfo
	for _, templateName := range templateNames {
		manifestInfo, err := GetTemplateInfo(templateName)
		if err != nil {
			manifestInfo = ManifestInfo{
				Name:        templateName,
				Description: "No description available",
				FilePath:    fmt.Sprintf("%s.yaml", templateName),
			}
		}
		manifests = append(manifests, manifestInfo)
	}

	// Print manifests in tabular format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tDESCRIPTION")
	for _, manifest := range manifests {
		fmt.Fprintf(w, "%s\t%s\n", manifest.Name, manifest.Description)
	}

	return w.Flush()
}

func runGetTemplate(cmd *cobra.Command, args []string) error {
	templateName := args[0]

	if !TemplateExists(templateName) {
		return fmt.Errorf("template '%s' not found", templateName)
	}

	if outputFormat == "yaml" || outputFormat == "" {
		content, err := GetTemplateContent(templateName)
		if err != nil {
			return fmt.Errorf("failed to read template: %v", err)
		}
		fmt.Print(content)
		return nil
	}

	// For other output formats, show template info
	manifestInfo, err := GetTemplateInfo(templateName)
	if err != nil {
		return fmt.Errorf("failed to get template info: %v", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tDESCRIPTION")
	fmt.Fprintf(w, "%s\t%s\n", manifestInfo.Name, manifestInfo.Description)
	return w.Flush()
}