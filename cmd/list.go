package cmd

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/mars/vela/pkg/helm"
	"github.com/mars/vela/pkg/kube"
	"github.com/spf13/cobra"
)

type listAppOutput struct {
	Name      string          `json:"name"`
	Namespace string          `json:"namespace"`
	Status    string          `json:"status"`
	Revision  int             `json:"revision"`
	Pods      []podOutput     `json:"pods,omitempty"`
	Services  []serviceOutput `json:"services,omitempty"`
	Ingresses []ingressOutput `json:"ingresses,omitempty"`
}

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

	kc, kubeErr := kube.New(kubeconfigVal, ns, insecure)

	if isJSON() {
		apps := make([]listAppOutput, 0, len(releases))
		for _, r := range releases {
			app := listAppOutput{
				Name:      r.Name,
				Namespace: r.Namespace,
				Status:    r.Status,
				Revision:  r.Revision,
			}
			if kc != nil && kubeErr == nil {
				pods, _ := kc.GetPods(context.Background(), r.Name)
				for _, p := range pods {
					app.Pods = append(app.Pods, podOutput{Name: p.Name, Status: p.Status, Ready: p.Ready})
				}
				services, _ := kc.GetServices(context.Background(), r.Name)
				for _, s := range services {
					app.Services = append(app.Services, serviceOutput{
						Name: s.Name, Type: s.Type, ClusterIP: s.ClusterIP, Ports: s.Ports,
					})
				}
				ingresses, _ := kc.GetIngresses(context.Background(), r.Name)
				for _, ing := range ingresses {
					app.Ingresses = append(app.Ingresses, ingressOutput{
						Name: ing.Name, Host: ing.Host, Path: ing.Path, URL: ing.URL,
					})
				}
			}
			apps = append(apps, app)
		}
		return writeJSON(cmd.OutOrStdout(), apps)
	}

	if len(releases) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No applications found in cluster.")
		return nil
	}

	for _, r := range releases {
		fmt.Fprintf(cmd.OutOrStdout(), "Release: %s  Namespace: %s  Status: %s  Revision: %d\n", r.Name, r.Namespace, r.Status, r.Revision)

		if kc == nil || kubeErr != nil {
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
