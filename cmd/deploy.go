// cmd/deploy.go
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/mars/vela/pkg/chart"
	"github.com/mars/vela/pkg/config"
	"github.com/mars/vela/pkg/helm"
	"github.com/mars/vela/pkg/project"
	"github.com/mars/vela/pkg/state"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy the application to the cluster",
	Args:  cobra.NoArgs,
	RunE:  runDeploy,
}

func runDeploy(cmd *cobra.Command, args []string) error {
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

	ts, err := config.Parse("tech-stack.yaml")
	if err != nil {
		return fmt.Errorf("parse tech-stack: %w", err)
	}

	chartDir := project.ChartDir(projectDir)
	if err := os.RemoveAll(chartDir); err != nil {
		return fmt.Errorf("clean chart dir: %w", err)
	}
	if err := os.MkdirAll(chartDir, 0755); err != nil {
		return fmt.Errorf("create chart dir: %w", err)
	}

	if err := chart.Generate(ts, chartDir); err != nil {
		return fmt.Errorf("generate chart: %w", err)
	}

	kubeconfigVal := cmd.Flag("kubeconfig").Value.String()
	ns := cmd.Flag("namespace").Value.String()
	hc := helm.New(kubeconfigVal, ns)
	name := ts.ProjectName()

	if hc.ReleaseExists(name) {
		fmt.Fprintf(cmd.OutOrStdout(), "Upgrading %q in namespace %q...\n", name, ns)
		if err := hc.Upgrade(name, chartDir); err != nil {
			st.Status = state.StatusFailed
			backend.Save(projectDir, st)
			return fmt.Errorf("upgrade failed: %w", err)
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Deploying %q to namespace %q...\n", name, ns)
		if err := hc.Install(name, chartDir); err != nil {
			st.Status = state.StatusFailed
			backend.Save(projectDir, st)
			return fmt.Errorf("deploy failed: %w", err)
		}
	}

	rel, _ := hc.Status(name)
	st.Status = state.StatusDeployed
	st.Namespace = ns
	st.Cluster = kubeconfigVal
	st.LastDeployed = time.Now().UTC().Format(time.RFC3339)
	if rel != nil {
		st.Revision = rel.Revision
	}

	st.Services = make(map[string]state.ServiceState)
	for _, svc := range ts.Services {
		ss := state.ServiceState{Image: svc.Image}
		if svc.Ingress != nil && svc.Ingress.Enabled {
			ss.IngressPath = svc.Ingress.Path
		}
		st.Services[svc.Name] = ss
	}
	backend.Save(projectDir, st)

	fmt.Fprintf(cmd.OutOrStdout(), "App %q deployed successfully.\n", name)
	fmt.Fprintln(cmd.OutOrStdout(), "Run 'vela status' to check deployment status.")
	return nil
}
