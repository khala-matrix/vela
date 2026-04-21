package app

import (
	"fmt"

	"github.com/mars/vela/pkg/helm"
	"github.com/mars/vela/pkg/store"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete an application from the cluster and local storage",
	Args:  cobra.ExactArgs(1),
	RunE:  runDelete,
}

func init() {
	AppCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
	name := args[0]
	s := store.New(store.DefaultBaseDir())
	kubeconfig := cmd.Flag("kubeconfig").Value.String()
	namespace := cmd.Flag("namespace").Value.String()
	hc := helm.New(kubeconfig, namespace)

	if hc.ReleaseExists(name) {
		fmt.Fprintf(cmd.OutOrStdout(), "Uninstalling release %q...\n", name)
		if err := hc.Uninstall(name); err != nil {
			return fmt.Errorf("uninstall release: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Release %q uninstalled.\n", name)
	}

	if s.Exists(name) {
		if err := s.Delete(name); err != nil {
			return fmt.Errorf("delete local files: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Local files for %q removed.\n", name)
	} else if !hc.ReleaseExists(name) {
		return fmt.Errorf("app %q not found locally or in cluster", name)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "App %q deleted successfully.\n", name)
	return nil
}
