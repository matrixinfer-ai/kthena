/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "minfer",
	Short: "MatrixInfer CLI for managing AI inference workloads",
	Long: `minfer is a CLI tool for managing MatrixInfer AI inference workloads.

It allows you to:
- Create manifests from predefined templates with custom values
- List and view MatrixInfer resources in Kubernetes clusters
- Manage inference workloads, models, and autoscaling policies

Examples:
  minfer get templates
  minfer describe template deepseek-r1-distill-llama-8b
  minfer get template deepseek-r1-distill-llama-8b -o yaml
  minfer create manifest --name my-model --template deepseek-r1-distill-llama-8b
  minfer get models
  minfer get modelinfers --all-namespaces`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.minfer.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
