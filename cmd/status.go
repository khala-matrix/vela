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

type statusOutput struct {
	Name      string          `json:"name"`
	Namespace string          `json:"namespace"`
	Status    string          `json:"status"`
	Revision  int             `json:"revision"`
	Updated   string          `json:"updated,omitempty"`
	Pods      []podOutput     `json:"pods"`
	Services  []serviceOutput `json:"services,omitempty"`
	Ingresses []ingressOutput `json:"ingresses,omitempty"`
}

type podOutput struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Ready  bool   `json:"ready"`
}

type serviceOutput struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	ClusterIP string `json:"cluster_ip"`
	Ports     string `json:"ports"`
}

type ingressOutput struct {
	Name string `json:"name"`
	Host string `json:"host,omitempty"`
	Path string `json:"path,omitempty"`
	URL  string `json:"url"`
}

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

	hc := helm.New(kubeconfigVal, ns, insecure)
	rel, err := hc.Status(st.Name)
	if err != nil {
		if isJSON() {
			return writeJSON(cmd.OutOrStdout(), map[string]any{
				"name":   st.Name,
				"status": "not_deployed",
			})
		}
		fmt.Fprintf(cmd.OutOrStdout(), "App %q is not deployed to the cluster.\n", st.Name)
		fmt.Fprintln(cmd.OutOrStdout(), "Run 'vela deploy' to deploy.")
		return nil
	}

	st.Status = state.StatusDeployed
	st.Namespace = rel.Namespace
	st.Revision = rel.Revision
	backend.Save(projectDir, st)

	kc, err := kube.New(kubeconfigVal, ns, insecure)
	if err != nil {
		return fmt.Errorf("create kube client: %w", err)
	}

	pods, err := kc.GetPods(context.Background(), st.Name)
	if err != nil {
		return fmt.Errorf("get pods: %w", err)
	}

	services, _ := kc.GetServices(context.Background(), st.Name)
	ingresses, _ := kc.GetIngresses(context.Background(), st.Name)

	if isJSON() {
		out := statusOutput{
			Name:      rel.Name,
			Namespace: rel.Namespace,
			Status:    rel.Status,
			Revision:  rel.Revision,
			Pods:      make([]podOutput, len(pods)),
		}
		if !rel.Updated.IsZero() {
			out.Updated = rel.Updated.Format("2006-01-02 15:04:05")
		}
		for i, p := range pods {
			out.Pods[i] = podOutput{Name: p.Name, Status: p.Status, Ready: p.Ready}
		}
		for _, s := range services {
			out.Services = append(out.Services, serviceOutput{
				Name: s.Name, Type: s.Type, ClusterIP: s.ClusterIP, Ports: s.Ports,
			})
		}
		for _, ing := range ingresses {
			out.Ingresses = append(out.Ingresses, ingressOutput{
				Name: ing.Name, Host: ing.Host, Path: ing.Path, URL: ing.URL,
			})
		}
		return writeJSON(cmd.OutOrStdout(), out)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Release:   %s\n", rel.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "Namespace: %s\n", rel.Namespace)
	fmt.Fprintf(cmd.OutOrStdout(), "Status:    %s\n", rel.Status)
	fmt.Fprintf(cmd.OutOrStdout(), "Revision:  %d\n", rel.Revision)
	if !rel.Updated.IsZero() {
		fmt.Fprintf(cmd.OutOrStdout(), "Updated:   %s\n", rel.Updated.Format("2006-01-02 15:04:05"))
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

	if len(services) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "\nServices:")
		w = tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTYPE\tCLUSTER-IP\tPORTS")
		for _, svc := range services {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", svc.Name, svc.Type, svc.ClusterIP, svc.Ports)
		}
		w.Flush()
	}

	if len(ingresses) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "\nIngress:")
		w = tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tURL")
		for _, ing := range ingresses {
			fmt.Fprintf(w, "%s\t%s\n", ing.Name, ing.URL)
		}
		w.Flush()
	}

	return nil
}
