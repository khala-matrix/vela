package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/mars/vela/pkg/helm"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all vela applications in the cluster",
	Args:  cobra.NoArgs,
	RunE:  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	kubeconfigVal := cmd.Flag("kubeconfig").Value.String()
	ns := cmd.Flag("namespace").Value.String()
	hc := helm.New(kubeconfigVal, ns)

	releases, err := hc.List()
	if err != nil {
		return fmt.Errorf("list releases: %w", err)
	}

	if len(releases) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No applications found in cluster.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tNAMESPACE\tSTATUS\tREVISION")
	for _, r := range releases {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", r.Name, r.Namespace, r.Status, r.Revision)
	}
	w.Flush()
	return nil
}
