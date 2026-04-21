package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mars/vela/pkg/chart"
	"github.com/mars/vela/pkg/config"
	"github.com/mars/vela/pkg/store"
	"github.com/spf13/cobra"
)

var createFile string

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new application from a tech-stack.yaml",
	Args:  cobra.ExactArgs(1),
	RunE:  runCreate,
}

func init() {
	createCmd.Flags().StringVarP(&createFile, "file", "f", "", "path to tech-stack.yaml (required)")
	createCmd.MarkFlagRequired("file")
	AppCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	name := args[0]
	s := store.New(store.DefaultBaseDir())

	if s.Exists(name) {
		return fmt.Errorf("app %q already exists, use 'vela app delete %s' first or 'vela app update %s -f <file>'", name, name, name)
	}

	ts, err := config.Parse(createFile)
	if err != nil {
		return fmt.Errorf("parse tech-stack: %w", err)
	}

	ts.Name = name

	chartDir := s.ChartDir(name)
	if err := os.MkdirAll(chartDir, 0755); err != nil {
		return fmt.Errorf("create chart directory: %w", err)
	}

	if err := chart.Generate(ts, chartDir); err != nil {
		return fmt.Errorf("generate chart: %w", err)
	}

	absPath, _ := filepath.Abs(createFile)
	meta := store.AppMeta{
		Name:          name,
		Namespace:     cmd.Flag("namespace").Value.String(),
		TechStackPath: absPath,
	}
	if err := s.Save(meta); err != nil {
		return fmt.Errorf("save app metadata: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "App %q created successfully. Chart generated at %s\n", name, chartDir)
	fmt.Fprintf(cmd.OutOrStdout(), "Run 'vela app deploy %s' to deploy to the cluster.\n", name)
	return nil
}
