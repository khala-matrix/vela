package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mars/vela/pkg/helm"
	"github.com/mars/vela/pkg/kube"
	"github.com/mars/vela/pkg/project"
	"github.com/mars/vela/pkg/state"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show application deployment status",
	Args:  cobra.NoArgs,
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
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

	hc := helm.New(kubeconfigVal, ns)
	rel, err := hc.Status(st.Name)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "App %q is not deployed to the cluster.\n", st.Name)
		fmt.Fprintln(cmd.OutOrStdout(), "Run 'vela deploy' to deploy.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Release:   %s\n", rel.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "Namespace: %s\n", rel.Namespace)
	fmt.Fprintf(cmd.OutOrStdout(), "Status:    %s\n", rel.Status)
	fmt.Fprintf(cmd.OutOrStdout(), "Revision:  %d\n", rel.Revision)
	if !rel.Updated.IsZero() {
		fmt.Fprintf(cmd.OutOrStdout(), "Updated:   %s\n", rel.Updated.Format("2006-01-02 15:04:05"))
	}

	st.Status = state.StatusDeployed
	st.Namespace = rel.Namespace
	st.Revision = rel.Revision
	backend.Save(projectDir, st)

	kc, err := kube.New(kubeconfigVal, ns)
	if err != nil {
		return fmt.Errorf("create kube client: %w", err)
	}

	pods, err := kc.GetPods(context.Background(), st.Name)
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
