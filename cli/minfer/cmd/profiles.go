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
	"io/ioutil"
	"os"
	"path/filepath"
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
	templatesDir := "templates"

	// Check if templates directory exists
	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		fmt.Printf("Templates directory '%s' not found.\n", templatesDir)
		fmt.Println("Please create the templates directory and add your profile templates.")
		return nil
	}

	// If describing a specific profile
	if describeProfile != "" {
		return describeProfileTemplate(describeProfile)
	}

	// List all profile templates
	files, err := ioutil.ReadDir(templatesDir)
	if err != nil {
		return fmt.Errorf("failed to read templates directory: %v", err)
	}

	var profiles []ProfileInfo
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".yaml") {
			profileName := strings.TrimSuffix(file.Name(), ".yaml")
			description, err := extractProfileDescription(filepath.Join(templatesDir, file.Name()))
			if err != nil {
				description = "No description available"
			}
			profiles = append(profiles, ProfileInfo{
				Name:        profileName,
				Description: description,
				FilePath:    file.Name(),
			})
		}
	}

	if len(profiles) == 0 {
		fmt.Printf("No profile templates found in '%s' directory.\n", templatesDir)
		fmt.Println("Profile templates should be YAML files with .yaml extension.")
		return nil
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

func extractProfileDescription(filePath string) (string, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Look for description in comments at the top of the file
		if strings.HasPrefix(trimmed, "# Description:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# Description:")), nil
		}
		if strings.HasPrefix(trimmed, "# ") && strings.Contains(strings.ToLower(trimmed), "description") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# ")), nil
		}
		// Stop looking after the first non-comment, non-empty line
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			break
		}
	}

	return "No description available", nil
}

func describeProfileTemplate(profileName string) error {
	templatePath := filepath.Join("templates", profileName+".yaml")

	// Check if template exists
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return fmt.Errorf("profile template '%s' not found", profileName)
	}

	// Read template content
	content, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template file: %v", err)
	}

	fmt.Printf("Profile: %s\n", profileName)
	fmt.Println("================")

	// Extract description
	description, _ := extractProfileDescription(templatePath)
	fmt.Printf("Description: %s\n\n", description)

	// Extract variables from template
	variables := extractTemplateVariables(string(content))
	if len(variables) > 0 {
		fmt.Println("Available Variables:")
		for _, variable := range variables {
			fmt.Printf("  - %s\n", variable)
		}
		fmt.Println()
	}

	fmt.Println("Template Content:")
	fmt.Println("=================")
	fmt.Println(string(content))

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