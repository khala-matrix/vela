package app

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/mars/vela/pkg/helm"
	"github.com/mars/vela/pkg/kube"
	"github.com/mars/vela/pkg/store"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status <name>",
	Short: "Show application deployment status",
	Args:  cobra.ExactArgs(1),
	RunE:  runStatus,
}

func init() {
	AppCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	name := args[0]
	kubeconfig := cmd.Flag("kubeconfig").Value.String()
	namespace := cmd.Flag("namespace").Value.String()
	s := store.New(store.DefaultBaseDir())

	if !s.Exists(name) {
		return fmt.Errorf("app %q not found locally", name)
	}

	hc := helm.New(kubeconfig, namespace)
	rel, err := hc.Status(name)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "App %q exists locally but is not deployed to the cluster.\n", name)
		fmt.Fprintf(cmd.OutOrStdout(), "Run 'vela app deploy %s' to deploy.\n", name)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Release:   %s\n", rel.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "Namespace: %s\n", rel.Namespace)
	fmt.Fprintf(cmd.OutOrStdout(), "Status:    %s\n", rel.Status)
	fmt.Fprintf(cmd.OutOrStdout(), "Revision:  %d\n", rel.Revision)
	if !rel.Updated.IsZero() {
		fmt.Fprintf(cmd.OutOrStdout(), "Updated:   %s\n", rel.Updated.Format("2006-01-02 15:04:05"))
	}

	kc, err := kube.New(kubeconfig, namespace)
	if err != nil {
		return fmt.Errorf("create kube client: %w", err)
	}

	pods, err := kc.GetPods(context.Background(), name)
	if err != nil {
		return fmt.Errorf("get pods: %w", err)
	}

	if len(pods) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "\nNo pods found.")
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nPods:")
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tREADY")
	for _, pod := range pods {
		ready := "No"
		if pod.Ready {
			ready = "Yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", pod.Name, pod.Status, ready)
	}
	w.Flush()
	return nil
}
