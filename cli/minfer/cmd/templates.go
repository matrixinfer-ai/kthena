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
	"embed"
	"fmt"
	"strings"
)

var templatesFS embed.FS

// InitTemplates initializes the templates filesystem
func InitTemplates(fs embed.FS) {
	templatesFS = fs
}

// GetTemplateContent returns the content of a template by name
func GetTemplateContent(templateName string) (string, error) {
	templatePath := fmt.Sprintf("templates/%s.yaml", templateName)
	content, err := templatesFS.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("template '%s' not found", templateName)
	}
	return string(content), nil
}

// ListTemplates returns a list of all available template names
func ListTemplates() ([]string, error) {
	entries, err := templatesFS.ReadDir("templates")
	if err != nil {
		return nil, fmt.Errorf("failed to read templates directory: %v", err)
	}

	var templates []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			templateName := strings.TrimSuffix(entry.Name(), ".yaml")
			templates = append(templates, templateName)
		}
	}

	return templates, nil
}

// TemplateExists checks if a template with the given name exists
func TemplateExists(templateName string) bool {
	templatePath := fmt.Sprintf("templates/%s.yaml", templateName)
	_, err := templatesFS.Open(templatePath)
	return err == nil
}

// GetTemplateInfo returns template information including name, description, and file path
func GetTemplateInfo(templateName string) (ManifestInfo, error) {
	content, err := GetTemplateContent(templateName)
	if err != nil {
		return ManifestInfo{}, err
	}

	description := extractManifestDescriptionFromContent(content)
	return ManifestInfo{
		Name:        templateName,
		Description: description,
		FilePath:    fmt.Sprintf("%s.yaml", templateName),
	}, nil
}

// extractManifestDescriptionFromContent extracts description from template content
func extractManifestDescriptionFromContent(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Look for description in comments at the top of the file
		if strings.HasPrefix(trimmed, "# Description:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# Description:"))
		}
		if strings.HasPrefix(trimmed, "# ") && strings.Contains(strings.ToLower(trimmed), "description") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
		// Stop looking after the first non-comment, non-empty line
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			break
		}
	}

	return "No description available"
}
