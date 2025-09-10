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

package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// manifestsCmd represents the manifests command
var manifestsCmd = &cobra.Command{
	Use:   "manifests",
	Short: "List available manifest templates",
	Long: `List all available manifest templates that can be used with the 'create manifest' command.

Manifests are predefined combinations of MatrixInfer resources that can be
customized with your specific values. Each manifest template defines a set
of resources and the variables that can be customized.

Examples:
  minfer manifests
  minfer manifests --describe basic-inference`,
	RunE: runListManifests,
}

func init() {
	// manifests command is now registered as a subcommand of list in list.go
}

func runListManifests(cmd *cobra.Command, args []string) error {
	// Get the describe flag value
	describeManifest, _ := cmd.Flags().GetString("describe")

	// If describing a specific manifest
	if describeManifest != "" {
		return describeManifestTemplate(describeManifest)
	}

	// List all manifest templates from embedded files
	templateNames, err := ListTemplates()
	if err != nil {
		return fmt.Errorf("failed to read templates: %v", err)
	}

	if len(templateNames) == 0 {
		fmt.Println("No manifest templates found.")
		return nil
	}

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

type ManifestInfo struct {
	Name        string
	Description string
	FilePath    string
}

func describeManifestTemplate(manifestName string) error {
	// Check if template exists
	if !TemplateExists(manifestName) {
		return fmt.Errorf("manifest template '%s' not found", manifestName)
	}

	// Read template content from embedded files
	content, err := GetTemplateContent(manifestName)
	if err != nil {
		return fmt.Errorf("failed to read template: %v", err)
	}

	fmt.Printf("Manifest: %s\n", manifestName)
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

func extractTemplateVariables(content string) []string {
	var variables []string
	variableMap := make(map[string]bool)

	// Simple regex-like approach to find {{ .VariableName }} patterns
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		for {
			start := strings.Index(line, "{{")
			if start == -1 {
				break
			}
			end := strings.Index(line[start:], "}}")
			if end == -1 {
				break
			}

			variable := strings.TrimSpace(line[start+2 : start+end])
			// Remove leading dot and any function calls/pipes
			variable = strings.TrimPrefix(variable, ".")
			if spaceIndex := strings.Index(variable, " "); spaceIndex != -1 {
				variable = variable[:spaceIndex]
			}
			if pipeIndex := strings.Index(variable, "|"); pipeIndex != -1 {
				variable = variable[:pipeIndex]
			}

			if variable != "" && !variableMap[variable] {
				variables = append(variables, variable)
				variableMap[variable] = true
			}

			line = line[start+end+2:]
		}
	}

	return variables
}
