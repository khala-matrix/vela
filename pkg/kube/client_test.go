package kube

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetPods(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "myapp-abc123",
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/instance": "myapp",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				},
			},
		},
	)

	c := NewFromClientset(clientset, "default")
	pods, err := c.GetPods(context.Background(), "myapp")
	if err != nil {
		t.Fatalf("GetPods failed: %v", err)
	}
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(pods))
	}
	if pods[0].Name != "myapp-abc123" {
		t.Errorf("expected pod name myapp-abc123, got %s", pods[0].Name)
	}
	if pods[0].Status != "Running" {
		t.Errorf("expected Running, got %s", pods[0].Status)
	}
	if !pods[0].Ready {
		t.Error("expected pod to be ready")
	}
}

func TestGetPods_NotReady(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "myapp-pending",
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/instance": "myapp",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionFalse},
				},
			},
		},
	)

	c := NewFromClientset(clientset, "default")
	pods, err := c.GetPods(context.Background(), "myapp")
	if err != nil {
		t.Fatalf("GetPods failed: %v", err)
	}
	if pods[0].Ready {
		t.Error("expected pod not ready")
	}
	if pods[0].Status != "Pending" {
		t.Errorf("expected Pending, got %s", pods[0].Status)
	}
}

func TestGetPods_Empty(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	c := NewFromClientset(clientset, "default")
	pods, err := c.GetPods(context.Background(), "noapp")
	if err != nil {
		t.Fatalf("GetPods failed: %v", err)
	}
	if len(pods) != 0 {
		t.Errorf("expected 0 pods, got %d", len(pods))
	}
}
