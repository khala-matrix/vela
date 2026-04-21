package app

import (
	"fmt"
	"text/tabwriter"

	"github.com/mars/vela/pkg/helm"
	"github.com/mars/vela/pkg/store"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all applications",
	Args:  cobra.NoArgs,
	RunE:  runList,
}

func init() {
	AppCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	s := store.New(store.DefaultBaseDir())

	apps, err := s.List()
	if err != nil {
		return fmt.Errorf("list apps: %w", err)
	}

	if len(apps) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No apps found. Create one with 'vela app create <name> -f <tech-stack.yaml>'")
		return nil
	}

	kubeconfig := cmd.Flag("kubeconfig").Value.String()
	namespace := cmd.Flag("namespace").Value.String()
	hc := helm.New(kubeconfig, namespace)

	releases, _ := hc.List()
	releaseMap := make(map[string]helm.ReleaseInfo)
	for _, r := range releases {
		releaseMap[r.Name] = r
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tNAMESPACE\tCLUSTER STATUS\tREVISION")
	for _, app := range apps {
		status := "not deployed"
		revision := "-"
		if rel, ok := releaseMap[app.Name]; ok {
			status = rel.Status
			revision = fmt.Sprintf("%d", rel.Revision)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", app.Name, app.Namespace, status, revision)
	}
	w.Flush()
	return nil
}
