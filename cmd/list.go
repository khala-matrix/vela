package cmd

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/mars/vela/pkg/helm"
	"github.com/mars/vela/pkg/kube"
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
	hc := helm.New(kubeconfigVal, ns, insecure)

	releases, err := hc.List()
	if err != nil {
		return fmt.Errorf("list releases: %w", err)
	}

	if len(releases) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No applications found in cluster.")
		return nil
	}

	kc, err := kube.New(kubeconfigVal, ns, insecure)

	for _, r := range releases {
		fmt.Fprintf(cmd.OutOrStdout(), "Release: %s  Namespace: %s  Status: %s  Revision: %d\n", r.Name, r.Namespace, r.Status, r.Revision)

		if kc == nil || err != nil {
			fmt.Fprintln(cmd.OutOrStdout())
			continue
		}

		pods, _ := kc.GetPods(context.Background(), r.Name)
		if len(pods) > 0 {
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  POD\tSTATUS\tREADY")
			for _, pod := range pods {
				ready := "No"
				if pod.Ready {
					ready = "Yes"
				}
				fmt.Fprintf(w, "  %s\t%s\t%s\n", pod.Name, pod.Status, ready)
			}
			w.Flush()
		}

		services, _ := kc.GetServices(context.Background(), r.Name)
		if len(services) > 0 {
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  SERVICE\tTYPE\tCLUSTER-IP\tPORTS")
			for _, svc := range services {
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", svc.Name, svc.Type, svc.ClusterIP, svc.Ports)
			}
			w.Flush()
		}

		ingresses, _ := kc.GetIngresses(context.Background(), r.Name)
		if len(ingresses) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "  Ingress:")
			for _, ing := range ingresses {
				fmt.Fprintf(cmd.OutOrStdout(), "    %s → %s\n", ing.Name, ing.URL)
			}
		}

		fmt.Fprintln(cmd.OutOrStdout())
	}
	return nil
}
