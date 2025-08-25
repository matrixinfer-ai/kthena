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
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// profilesCmd represents the profiles command
var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "List available profile templates",
	Long: `List all available profile templates that can be used with the 'create profile' command.

Profiles are predefined combinations of MatrixInfer resources that can be
customized with your specific values. Each profile template defines a set
of resources and the variables that can be customized.

Examples:
  minfer profiles
  minfer profiles --describe basic-inference`,
	RunE: runListProfiles,
}

var describeProfile string

func init() {
	rootCmd.AddCommand(profilesCmd)
	profilesCmd.Flags().StringVar(&describeProfile, "describe", "", "Show detailed information about a specific profile template")
}

func runListProfiles(cmd *cobra.Command, args []string) error {
	// If describing a specific profile
	if describeProfile != "" {
		return describeProfileTemplate(describeProfile)
	}

	// List all profile templates from embedded files
	templateNames, err := ListTemplates()
	if err != nil {
		return fmt.Errorf("failed to read templates: %v", err)
	}

	if len(templateNames) == 0 {
		fmt.Println("No profile templates found.")
		return nil
	}

	var profiles []ProfileInfo
	for _, templateName := range templateNames {
		profileInfo, err := GetTemplateInfo(templateName)
		if err != nil {
			profileInfo = ProfileInfo{
				Name:        templateName,
				Description: "No description available",
				FilePath:    fmt.Sprintf("%s.yaml", templateName),
			}
		}
		profiles = append(profiles, profileInfo)
	}

	// Print profiles in tabular format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tDESCRIPTION")
	for _, profile := range profiles {
		fmt.Fprintf(w, "%s\t%s\n", profile.Name, profile.Description)
	}

	return w.Flush()
}

type ProfileInfo struct {
	Name        string
	Description string
	FilePath    string
}


func describeProfileTemplate(profileName string) error {
	// Check if template exists
	if !TemplateExists(profileName) {
		return fmt.Errorf("profile template '%s' not found", profileName)
	}

	// Read template content from embedded files
	content, err := GetTemplateContent(profileName)
	if err != nil {
		return fmt.Errorf("failed to read template: %v", err)
	}

	fmt.Printf("Profile: %s\n", profileName)
	fmt.Println("================")

	// Extract description
	description := extractProfileDescriptionFromContent(content)
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