package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/mars/vela/pkg/kube"
	"github.com/mars/vela/pkg/project"
	"github.com/mars/vela/pkg/state"
	"github.com/spf13/cobra"
)

var (
	logsFollow bool
	logsAll    bool
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View application pod logs",
	Args:  cobra.NoArgs,
	RunE:  runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "F", false, "follow log output")
	logsCmd.Flags().BoolVar(&logsAll, "all", false, "show logs from all pods")
}

func runLogs(cmd *cobra.Command, args []string) error {
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

	kc, err := kube.New(kubeconfigVal, ns)
	if err != nil {
		return fmt.Errorf("create kube client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	pods, err := kc.GetPods(ctx, st.Name)
	if err != nil {
		return fmt.Errorf("get pods: %w", err)
	}

	if len(pods) == 0 {
		return fmt.Errorf("no pods found for app %q", st.Name)
	}

	targetPods := pods
	if !logsAll {
		targetPods = pods[:1]
	}

	for _, pod := range targetPods {
		if len(targetPods) > 1 {
			fmt.Fprintf(cmd.OutOrStdout(), "=== %s ===\n", pod.Name)
		}

		stream, err := kc.GetPodLogs(ctx, pod.Name, logsFollow)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error getting logs for %s: %v\n", pod.Name, err)
			continue
		}

		io.Copy(cmd.OutOrStdout(), stream)
		stream.Close()
	}

	return nil
}
