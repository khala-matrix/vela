package kube

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type PodInfo struct {
	Name   string
	Status string
	Ready  bool
}

type Client struct {
	clientset kubernetes.Interface
	namespace string
}

func New(kubeconfig, namespace string) (*Client, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("build kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}

	return &Client{clientset: clientset, namespace: namespace}, nil
}

func NewFromClientset(clientset kubernetes.Interface, namespace string) *Client {
	return &Client{clientset: clientset, namespace: namespace}
}

func (c *Client) GetPods(ctx context.Context, appName string) ([]PodInfo, error) {
	selector := fmt.Sprintf("app.kubernetes.io/instance=%s", appName)
	pods, err := c.clientset.CoreV1().Pods(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	infos := make([]PodInfo, 0, len(pods.Items))
	for _, pod := range pods.Items {
		ready := true
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status != corev1.ConditionTrue {
				ready = false
				break
			}
		}
		infos = append(infos, PodInfo{
			Name:   pod.Name,
			Status: string(pod.Status.Phase),
			Ready:  ready,
		})
	}
	return infos, nil
}

func (c *Client) GetPodLogs(ctx context.Context, podName string, follow bool) (io.ReadCloser, error) {
	opts := &corev1.PodLogOptions{
		Follow:    follow,
		TailLines: int64Ptr(100),
	}

	req := c.clientset.CoreV1().Pods(c.namespace).GetLogs(podName, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("stream logs for pod %q: %w", podName, err)
	}
	return stream, nil
}

func int64Ptr(i int64) *int64 {
	return &i
}
