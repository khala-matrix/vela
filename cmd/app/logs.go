package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/mars/vela/pkg/kube"
	"github.com/mars/vela/pkg/store"
	"github.com/spf13/cobra"
)

var (
	logsFollow bool
	logsAll    bool
)

var logsCmd = &cobra.Command{
	Use:   "logs <name>",
	Short: "View application pod logs",
	Args:  cobra.ExactArgs(1),
	RunE:  runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "F", false, "follow log output")
	logsCmd.Flags().BoolVar(&logsAll, "all", false, "show logs from all pods")
	AppCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	name := args[0]
	kubeconfig := cmd.Flag("kubeconfig").Value.String()
	namespace := cmd.Flag("namespace").Value.String()
	s := store.New(store.DefaultBaseDir())

	if !s.Exists(name) {
		return fmt.Errorf("app %q not found", name)
	}

	kc, err := kube.New(kubeconfig, namespace)
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

	pods, err := kc.GetPods(ctx, name)
	if err != nil {
		return fmt.Errorf("get pods: %w", err)
	}

	if len(pods) == 0 {
		return fmt.Errorf("no pods found for app %q", name)
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
