/*
Copyright The Volcano Authors.

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
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// describeCmd represents the describe command
var describeCmd = &cobra.Command{
	Use:   "describe",
	Short: "Show detailed information about a specific resource",
	Long: `Show detailed information about a specific resource.

You can describe templates and other kthena resources.

Examples:
  minfer describe template deepseek-r1-distill-llama-8b
  minfer describe model my-model
  minfer describe modelinfer my-inference
  minfer describe autoscaling-policy my-policy`,
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

// describeModelCmd represents the describe model command
var describeModelCmd = &cobra.Command{
	Use:   "model [NAME]",
	Short: "Show detailed information about a model",
	Long: `Show detailed information about a specific Model resource in the cluster.

This will display the model configuration, status, and resource details.`,
	Args: cobra.ExactArgs(1),
	RunE: runDescribeModel,
}

// describeModelInferCmd represents the describe modelinfer command
var describeModelInferCmd = &cobra.Command{
	Use:     "modelinfer [NAME]",
	Aliases: []string{"mi"},
	Short:   "Show detailed information about a model inference workload",
	Long: `Show detailed information about a specific ModelInfer resource in the cluster.

This will display the modelinfer configuration, status, and resource details.`,
	Args: cobra.ExactArgs(1),
	RunE: runDescribeModelInfer,
}

// describeAutoscalingPolicyCmd represents the describe autoscaling-policy command
var describeAutoscalingPolicyCmd = &cobra.Command{
	Use:     "autoscaling-policy [NAME]",
	Aliases: []string{"asp"},
	Short:   "Show detailed information about an autoscaling policy",
	Long: `Show detailed information about a specific AutoscalingPolicy resource in the cluster.

This will display the autoscaling policy configuration and rules.`,
	Args: cobra.ExactArgs(1),
	RunE: runDescribeAutoscalingPolicy,
}

func init() {
	rootCmd.AddCommand(describeCmd)
	describeCmd.AddCommand(describeTemplateCmd)
	describeCmd.AddCommand(describeModelCmd)
	describeCmd.AddCommand(describeModelInferCmd)
	describeCmd.AddCommand(describeAutoscalingPolicyCmd)

	// Add namespace flags
	describeCmd.PersistentFlags().StringVarP(&getNamespace, "namespace", "n", "", "Kubernetes namespace (default: current context namespace)")
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
	fmt.Println("=================")
	fmt.Println("Template Content:")
	fmt.Println("=================")
	fmt.Println(content)

	return nil
}

func runDescribeModel(cmd *cobra.Command, args []string) error {
	modelName := args[0]

	client, err := getKthenaClient()
	if err != nil {
		return err
	}

	namespace := getNamespace
	if namespace == "" {
		namespace = "default"
	}
	ctx := context.Background()

	model, err := client.RegistryV1alpha1().Models(namespace).Get(ctx, modelName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get Model '%s': %v", modelName, err)
	}

	fmt.Printf("Model: %s\n", model.Name)
	fmt.Println("================")
	fmt.Printf("Namespace: %s\n", model.Namespace)
	fmt.Printf("Created: %s\n", model.CreationTimestamp.Time.Format(time.RFC3339))
	fmt.Printf("Age: %s\n\n", time.Since(model.CreationTimestamp.Time).Truncate(time.Second))

	// Output the full resource as YAML
	data, err := yaml.Marshal(model)
	if err != nil {
		return fmt.Errorf("failed to marshal Model to YAML: %v", err)
	}

	fmt.Println("Resource Details:")
	fmt.Println("=================")
	fmt.Print(string(data))

	return nil
}

func runDescribeModelInfer(cmd *cobra.Command, args []string) error {
	modelInferName := args[0]

	client, err := getKthenaClient()
	if err != nil {
		return err
	}

	namespace := getNamespace
	if namespace == "" {
		namespace = "default"
	}
	ctx := context.Background()

	modelInfer, err := client.WorkloadV1alpha1().ModelInfers(namespace).Get(ctx, modelInferName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ModelInfer '%s': %v", modelInferName, err)
	}

	fmt.Printf("ModelInfer: %s\n", modelInfer.Name)
	fmt.Println("================")
	fmt.Printf("Namespace: %s\n", modelInfer.Namespace)
	fmt.Printf("Created: %s\n", modelInfer.CreationTimestamp.Time.Format(time.RFC3339))
	fmt.Printf("Age: %s\n\n", time.Since(modelInfer.CreationTimestamp.Time).Truncate(time.Second))

	// Output the full resource as YAML
	data, err := yaml.Marshal(modelInfer)
	if err != nil {
		return fmt.Errorf("failed to marshal ModelInfer to YAML: %v", err)
	}

	fmt.Println("Resource Details:")
	fmt.Println("=================")
	fmt.Print(string(data))

	return nil
}

func runDescribeAutoscalingPolicy(cmd *cobra.Command, args []string) error {
	policyName := args[0]

	client, err := getKthenaClient()
	if err != nil {
		return err
	}

	namespace := getNamespace
	if namespace == "" {
		namespace = "default"
	}
	ctx := context.Background()

	policy, err := client.RegistryV1alpha1().AutoscalingPolicies(namespace).Get(ctx, policyName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get AutoscalingPolicy '%s': %v", policyName, err)
	}

	fmt.Printf("AutoscalingPolicy: %s\n", policy.Name)
	fmt.Println("================")
	fmt.Printf("Namespace: %s\n", policy.Namespace)
	fmt.Printf("Created: %s\n", policy.CreationTimestamp.Time.Format(time.RFC3339))
	fmt.Printf("Age: %s\n\n", time.Since(policy.CreationTimestamp.Time).Truncate(time.Second))

	// Output the full resource as YAML
	data, err := yaml.Marshal(policy)
	if err != nil {
		return fmt.Errorf("failed to marshal AutoscalingPolicy to YAML: %v", err)
	}

	fmt.Println("Resource Details:")
	fmt.Println("=================")
	fmt.Print(string(data))

	return nil
}
