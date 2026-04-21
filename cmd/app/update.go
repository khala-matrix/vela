package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mars/vela/pkg/chart"
	"github.com/mars/vela/pkg/config"
	"github.com/mars/vela/pkg/helm"
	"github.com/mars/vela/pkg/store"
	"github.com/spf13/cobra"
)

var updateFile string

var updateCmd = &cobra.Command{
	Use:   "update <name>",
	Short: "Update an application: re-render chart and helm upgrade",
	Args:  cobra.ExactArgs(1),
	RunE:  runUpdate,
}

func init() {
	updateCmd.Flags().StringVarP(&updateFile, "file", "f", "", "path to updated tech-stack.yaml (required)")
	updateCmd.MarkFlagRequired("file")
	AppCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	name := args[0]
	s := store.New(store.DefaultBaseDir())

	if !s.Exists(name) {
		return fmt.Errorf("app %q not found, run 'vela app create %s -f <tech-stack.yaml>' first", name, name)
	}

	ts, err := config.Parse(updateFile)
	if err != nil {
		return fmt.Errorf("parse tech-stack: %w", err)
	}

	ts.Name = name

	chartDir := s.ChartDir(name)
	if err := os.RemoveAll(chartDir); err != nil {
		return fmt.Errorf("clean old chart: %w", err)
	}
	if err := os.MkdirAll(chartDir, 0755); err != nil {
		return fmt.Errorf("create chart directory: %w", err)
	}

	if err := chart.Generate(ts, chartDir); err != nil {
		return fmt.Errorf("generate chart: %w", err)
	}

	absPath, _ := filepath.Abs(updateFile)
	meta := store.AppMeta{
		Name:          name,
		Namespace:     cmd.Flag("namespace").Value.String(),
		TechStackPath: absPath,
	}
	if err := s.Save(meta); err != nil {
		return fmt.Errorf("save app metadata: %w", err)
	}

	kubeconfig := cmd.Flag("kubeconfig").Value.String()
	namespace := cmd.Flag("namespace").Value.String()
	hc := helm.New(kubeconfig, namespace)

	fmt.Fprintf(cmd.OutOrStdout(), "Updating %q in namespace %q...\n", name, namespace)

	if err := hc.Upgrade(name, chartDir); err != nil {
		return fmt.Errorf("upgrade failed: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "App %q updated successfully.\n", name)
	return nil
}
