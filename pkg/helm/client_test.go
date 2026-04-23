// pkg/helm/client_test.go
package helm

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	c := New("/tmp/fake-kubeconfig", "default", false)
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.namespace != "default" {
		t.Errorf("expected namespace default, got %s", c.namespace)
	}
	if c.kubeconfig != "/tmp/fake-kubeconfig" {
		t.Errorf("expected kubeconfig /tmp/fake-kubeconfig, got %s", c.kubeconfig)
	}
}

func TestReleaseExists_NoCluster(t *testing.T) {
	c := New("/tmp/nonexistent-kubeconfig", "default", false)
	if c.ReleaseExists("nonexistent") {
		t.Error("expected ReleaseExists to return false without a cluster")
	}
}
