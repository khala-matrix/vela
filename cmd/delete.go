package cmd

import (
	"fmt"
	"os"

	"github.com/mars/vela/pkg/helm"
	"github.com/mars/vela/pkg/project"
	"github.com/mars/vela/pkg/state"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete the application from the cluster",
	Args:  cobra.NoArgs,
	RunE:  runDelete,
}

func runDelete(cmd *cobra.Command, args []string) error {
	cwd, _ := os.Getwd()
	projectDir, err := project.Find(cwd)
	if err != nil {
		return err
	}

	backend := &state.LocalBackend{}
	st, err := backend.Load(projectDir)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	kubeconfigVal := cmd.Flag("kubeconfig").Value.String()
	ns := cmd.Flag("namespace").Value.String()
	hc := helm.New(kubeconfigVal, ns, insecure)

	if hc.ReleaseExists(st.Name) {
		fmt.Fprintf(cmd.OutOrStdout(), "Uninstalling release %q...\n", st.Name)
		if err := hc.Uninstall(st.Name); err != nil {
			return fmt.Errorf("uninstall release: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Release %q uninstalled.\n", st.Name)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Release %q not found in cluster.\n", st.Name)
	}

	st.Status = state.StatusDeleted
	backend.Save(projectDir, st)

	fmt.Fprintf(cmd.OutOrStdout(), "App %q deleted.\n", st.Name)
	return nil
}
