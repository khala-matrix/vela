package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	kubeconfig string
	namespace  string
	verbose    bool
	insecure   bool
)

var rootCmd = &cobra.Command{
	Use:   "vela",
	Short: "Deploy applications to k3s clusters",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	defaultKubeconfig := os.Getenv("KUBECONFIG")
	if defaultKubeconfig == "" {
		home, _ := os.UserHomeDir()
		defaultKubeconfig = filepath.Join(home, ".kube", "config")
	}

	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", defaultKubeconfig, "path to kubeconfig file")
	rootCmd.PersistentFlags().StringVar(&namespace, "namespace", "sandbox", "target namespace")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&insecure, "insecure", false, "skip TLS certificate verification")

	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(configureCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(credentialsCmd)
	rootCmd.AddCommand(guideCmd)
}
