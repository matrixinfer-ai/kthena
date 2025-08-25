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
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
)

var (
	listNamespace string
	allNamespaces bool
	describeManifest string
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List MatrixInfer resources",
	Long: `List MatrixInfer resources in the Kubernetes cluster.

Available resource types:
- modelinfers: List model inference workloads
- models: List registered models
- autoscaling-policies: List autoscaling policies
- autoscaling-policy-bindings: List autoscaling policy bindings
- templates: List available manifest templates

Examples:
  minfer list modelinfers
  minfer list models
  minfer list models --namespace=production
  minfer list modelinfers --all-namespaces
  minfer list templates
  minfer list templates --describe basic-inference`,
}

// modelinfersCmd represents the list modelinfers command
var modelinfersCmd = &cobra.Command{
	Use:     "modelinfers",
	Aliases: []string{"mi", "modelinfer"},
	Short:   "List model inference workloads",
	Long:    `List ModelInfer resources in the cluster.`,
	RunE:    runListModelInfers,
}

// modelsCmd represents the list models command
var modelsCmd = &cobra.Command{
	Use:     "models",
	Aliases: []string{"model"},
	Short:   "List registered models",
	Long:    `List Model resources in the cluster.`,
	RunE:    runListModels,
}

// autoscalingPoliciesCmd represents the list autoscaling-policies command
var autoscalingPoliciesCmd = &cobra.Command{
	Use:     "autoscaling-policies",
	Aliases: []string{"asp", "autoscaling-policy"},
	Short:   "List autoscaling policies",
	Long:    `List AutoscalingPolicy resources in the cluster.`,
	RunE:    runListAutoscalingPolicies,
}

// autoscalingPolicyBindingsCmd represents the list autoscaling-policy-bindings command
var autoscalingPolicyBindingsCmd = &cobra.Command{
	Use:     "autoscaling-policy-bindings",
	Aliases: []string{"aspb", "autoscaling-policy-binding"},
	Short:   "List autoscaling policy bindings",
	Long:    `List AutoscalingPolicyBinding resources in the cluster.`,
	RunE:    runListAutoscalingPolicyBindings,
}

// templatesListCmd represents the list templates command
var templatesListCmd = &cobra.Command{
	Use:   "templates",
	Short: "List available manifest templates",
	Long: `List all available manifest templates that can be used with the 'create manifest' command.

Templates are predefined combinations of MatrixInfer resources that can be
customized with your specific values. Each template defines a set
of resources and the variables that can be customized.

Examples:
  minfer list templates
  minfer list templates --describe basic-inference`,
	RunE: runListProfiles,
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.AddCommand(modelinfersCmd)
	listCmd.AddCommand(modelsCmd)
	listCmd.AddCommand(autoscalingPoliciesCmd)
	listCmd.AddCommand(autoscalingPolicyBindingsCmd)
	listCmd.AddCommand(templatesListCmd)

	// Add persistent flags to the list command
	listCmd.PersistentFlags().StringVarP(&listNamespace, "namespace", "n", "", "Kubernetes namespace (default: current context namespace)")
	listCmd.PersistentFlags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "List resources across all namespaces")
	
	// Add templates-specific flags
	templatesListCmd.Flags().StringVar(&describeManifest, "describe", "", "Show detailed information about a specific template")
}

func getMatrixInferClient() (*versioned.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %v", err)
	}

	client, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create MatrixInfer client: %v", err)
	}

	return client, nil
}

func resolveNamespace() string {
	if allNamespaces {
		return ""
	}
	if listNamespace != "" {
		return listNamespace
	}
	return "default"
}

func runListModelInfers(cmd *cobra.Command, args []string) error {
	client, err := getMatrixInferClient()
	if err != nil {
		return err
	}

	namespace := resolveNamespace()
	ctx := context.Background()

	modelinfers, err := client.WorkloadV1alpha1().ModelInfers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list ModelInfers: %v", err)
	}

	if len(modelinfers.Items) == 0 {
		if allNamespaces {
			fmt.Println("No ModelInfers found across all namespaces.")
		} else {
			fmt.Printf("No ModelInfers found in namespace %s.\n", namespace)
		}
		return nil
	}

	// Print header
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if allNamespaces {
		fmt.Fprintln(w, "NAMESPACE\tNAME\tAGE")
	} else {
		fmt.Fprintln(w, "NAME\tAGE")
	}

	// Print ModelInfers
	for _, mi := range modelinfers.Items {
		age := time.Since(mi.CreationTimestamp.Time).Truncate(time.Second)
		if allNamespaces {
			fmt.Fprintf(w, "%s\t%s\t%s\n", mi.Namespace, mi.Name, age)
		} else {
			fmt.Fprintf(w, "%s\t%s\n", mi.Name, age)
		}
	}

	return w.Flush()
}

func runListModels(cmd *cobra.Command, args []string) error {
	client, err := getMatrixInferClient()
	if err != nil {
		return err
	}

	namespace := resolveNamespace()
	ctx := context.Background()

	models, err := client.RegistryV1alpha1().Models(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list Models: %v", err)
	}

	if len(models.Items) == 0 {
		if allNamespaces {
			fmt.Println("No Models found across all namespaces.")
		} else {
			fmt.Printf("No Models found in namespace %s.\n", namespace)
		}
		return nil
	}

	// Print header
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if allNamespaces {
		fmt.Fprintln(w, "NAMESPACE\tNAME\tAGE")
	} else {
		fmt.Fprintln(w, "NAME\tAGE")
	}

	// Print Models
	for _, model := range models.Items {
		age := time.Since(model.CreationTimestamp.Time).Truncate(time.Second)
		if allNamespaces {
			fmt.Fprintf(w, "%s\t%s\t%s\n", model.Namespace, model.Name, age)
		} else {
			fmt.Fprintf(w, "%s\t%s\n", model.Name, age)
		}
	}

	return w.Flush()
}

func runListAutoscalingPolicies(cmd *cobra.Command, args []string) error {
	client, err := getMatrixInferClient()
	if err != nil {
		return err
	}

	namespace := resolveNamespace()
	ctx := context.Background()

	policies, err := client.RegistryV1alpha1().AutoscalingPolicies(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list AutoscalingPolicies: %v", err)
	}

	if len(policies.Items) == 0 {
		if allNamespaces {
			fmt.Println("No AutoscalingPolicies found across all namespaces.")
		} else {
			fmt.Printf("No AutoscalingPolicies found in namespace %s.\n", namespace)
		}
		return nil
	}

	// Print header
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if allNamespaces {
		fmt.Fprintln(w, "NAMESPACE\tNAME\tAGE")
	} else {
		fmt.Fprintln(w, "NAME\tAGE")
	}

	// Print AutoscalingPolicies
	for _, policy := range policies.Items {
		age := time.Since(policy.CreationTimestamp.Time).Truncate(time.Second)
		if allNamespaces {
			fmt.Fprintf(w, "%s\t%s\t%s\n", policy.Namespace, policy.Name, age)
		} else {
			fmt.Fprintf(w, "%s\t%s\n", policy.Name, age)
		}
	}

	return w.Flush()
}

func runListAutoscalingPolicyBindings(cmd *cobra.Command, args []string) error {
	client, err := getMatrixInferClient()
	if err != nil {
		return err
	}

	namespace := resolveNamespace()
	ctx := context.Background()

	bindings, err := client.RegistryV1alpha1().AutoscalingPolicyBindings(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list AutoscalingPolicyBindings: %v", err)
	}

	if len(bindings.Items) == 0 {
		if allNamespaces {
			fmt.Println("No AutoscalingPolicyBindings found across all namespaces.")
		} else {
			fmt.Printf("No AutoscalingPolicyBindings found in namespace %s.\n", namespace)
		}
		return nil
	}

	// Print header
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if allNamespaces {
		fmt.Fprintln(w, "NAMESPACE\tNAME\tAGE")
	} else {
		fmt.Fprintln(w, "NAME\tAGE")
	}

	// Print AutoscalingPolicyBindings
	for _, binding := range bindings.Items {
		age := time.Since(binding.CreationTimestamp.Time).Truncate(time.Second)
		if allNamespaces {
			fmt.Fprintf(w, "%s\t%s\t%s\n", binding.Namespace, binding.Name, age)
		} else {
			fmt.Fprintf(w, "%s\t%s\n", binding.Name, age)
		}
	}

	return w.Flush()
}