package kube

import (
	"context"
	"fmt"
	"io"
	"strings"

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

type IngressInfo struct {
	Name  string
	Host  string
	Path  string
	URL   string
}

type ServiceInfo struct {
	Name      string
	Type      string
	ClusterIP string
	Ports     string
}

type Client struct {
	clientset kubernetes.Interface
	namespace string
}

func New(kubeconfig, namespace string, insecure bool) (*Client, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("build kubeconfig: %w", err)
	}

	if insecure {
		config.TLSClientConfig.Insecure = true
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

func (c *Client) GetIngresses(ctx context.Context, appName string) ([]IngressInfo, error) {
	selector := fmt.Sprintf("app.kubernetes.io/instance=%s", appName)
	ingresses, err := c.clientset.NetworkingV1().Ingresses(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, fmt.Errorf("list ingresses: %w", err)
	}

	var infos []IngressInfo
	for _, ing := range ingresses.Items {
		for _, rule := range ing.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}
			for _, path := range rule.HTTP.Paths {
				url := fmt.Sprintf("http://%s%s", rule.Host, path.Path)
				infos = append(infos, IngressInfo{
					Name: ing.Name,
					Host: rule.Host,
					Path: path.Path,
					URL:  url,
				})
			}
		}
	}
	return infos, nil
}

func (c *Client) GetServices(ctx context.Context, appName string) ([]ServiceInfo, error) {
	selector := fmt.Sprintf("app.kubernetes.io/instance=%s", appName)
	services, err := c.clientset.CoreV1().Services(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}

	var infos []ServiceInfo
	for _, svc := range services.Items {
		var ports []string
		for _, p := range svc.Spec.Ports {
			ports = append(ports, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
		}
		infos = append(infos, ServiceInfo{
			Name:      svc.Name,
			Type:      string(svc.Spec.Type),
			ClusterIP: svc.Spec.ClusterIP,
			Ports:     strings.Join(ports, ","),
		})
	}
	return infos, nil
}

func int64Ptr(i int64) *int64 {
	return &i
}
