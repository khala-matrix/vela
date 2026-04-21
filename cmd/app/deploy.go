package app

import (
	"fmt"

	"github.com/mars/vela/pkg/helm"
	"github.com/mars/vela/pkg/store"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy <name>",
	Short: "Deploy an application to the cluster",
	Args:  cobra.ExactArgs(1),
	RunE:  runDeploy,
}

func init() {
	AppCmd.AddCommand(deployCmd)
}

func runDeploy(cmd *cobra.Command, args []string) error {
	name := args[0]
	s := store.New(store.DefaultBaseDir())

	if !s.Exists(name) {
		return fmt.Errorf("app %q not found, run 'vela app create %s -f <tech-stack.yaml>' first", name, name)
	}

	kubeconfig := cmd.Flag("kubeconfig").Value.String()
	namespace := cmd.Flag("namespace").Value.String()

	hc := helm.New(kubeconfig, namespace)

	if hc.ReleaseExists(name) {
		return fmt.Errorf("release %q already exists, use 'vela app update %s' instead", name, name)
	}

	chartDir := s.ChartDir(name)
	fmt.Fprintf(cmd.OutOrStdout(), "Deploying %q to namespace %q...\n", name, namespace)

	if err := hc.Install(name, chartDir); err != nil {
		return fmt.Errorf("deploy failed: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "App %q deployed successfully.\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "Run 'vela app status %s' to check deployment status.\n", name)
	return nil
}
