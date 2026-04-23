package helm

import (
	"fmt"
	"log"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	settings   *cli.EnvSettings
	namespace  string
	kubeconfig string
	insecure   bool
}

type ReleaseInfo struct {
	Name      string
	Namespace string
	Status    string
	Revision  int
	Updated   time.Time
}

func New(kubeconfig, namespace string, insecure bool) *Client {
	settings := cli.New()
	settings.KubeConfig = kubeconfig
	settings.SetNamespace(namespace)
	return &Client{
		settings:   settings,
		namespace:  namespace,
		kubeconfig: kubeconfig,
		insecure:   insecure,
	}
}

func (c *Client) actionConfig() (*action.Configuration, error) {
	cfg := new(action.Configuration)
	logFunc := func(format string, v ...interface{}) {
		log.Printf(format, v...)
	}

	if c.insecure {
		getter := &insecureGetter{kubeconfig: c.kubeconfig, namespace: c.namespace}
		if err := cfg.Init(getter, c.namespace, "secret", logFunc); err != nil {
			return nil, fmt.Errorf("init helm config: %w", err)
		}
	} else {
		if err := cfg.Init(c.settings.RESTClientGetter(), c.namespace, "secret", logFunc); err != nil {
			return nil, fmt.Errorf("init helm config: %w", err)
		}
	}
	return cfg, nil
}

type insecureGetter struct {
	kubeconfig string
	namespace  string
}

func (g *insecureGetter) ToRESTConfig() (*rest.Config, error) {
	config, err := g.ToRawKubeConfigLoader().ClientConfig()
	if err != nil {
		return nil, err
	}
	config.TLSClientConfig.Insecure = true
	return config, nil
}

func (g *insecureGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	config, err := g.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	dc, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	return memory.NewMemCacheClient(dc), nil
}

func (g *insecureGetter) ToRESTMapper() (meta.RESTMapper, error) {
	dc, err := g.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	return restmapper.NewDeferredDiscoveryRESTMapper(dc), nil
}

func (g *insecureGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: g.kubeconfig}
	overrides := &clientcmd.ConfigOverrides{}
	overrides.ClusterInfo.InsecureSkipTLSVerify = true
	overrides.Context.Namespace = g.namespace
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
}

func (c *Client) Install(name, chartDir string) error {
	cfg, err := c.actionConfig()
	if err != nil {
		return err
	}

	chart, err := loader.Load(chartDir)
	if err != nil {
		return fmt.Errorf("load chart from %s: %w", chartDir, err)
	}

	install := action.NewInstall(cfg)
	install.ReleaseName = name
	install.Namespace = c.namespace
	install.CreateNamespace = true
	install.Wait = false

	if _, err := install.Run(chart, chart.Values); err != nil {
		return fmt.Errorf("install release %q: %w", name, err)
	}
	return nil
}

func (c *Client) Upgrade(name, chartDir string) error {
	cfg, err := c.actionConfig()
	if err != nil {
		return err
	}

	chart, err := loader.Load(chartDir)
	if err != nil {
		return fmt.Errorf("load chart from %s: %w", chartDir, err)
	}

	upgrade := action.NewUpgrade(cfg)
	upgrade.Namespace = c.namespace
	upgrade.Wait = false

	if _, err := upgrade.Run(name, chart, chart.Values); err != nil {
		return fmt.Errorf("upgrade release %q: %w", name, err)
	}
	return nil
}

func (c *Client) Uninstall(name string) error {
	cfg, err := c.actionConfig()
	if err != nil {
		return err
	}

	uninstall := action.NewUninstall(cfg)
	if _, err := uninstall.Run(name); err != nil {
		return fmt.Errorf("uninstall release %q: %w", name, err)
	}
	return nil
}

func (c *Client) Status(name string) (*ReleaseInfo, error) {
	cfg, err := c.actionConfig()
	if err != nil {
		return nil, err
	}

	status := action.NewStatus(cfg)
	rel, err := status.Run(name)
	if err != nil {
		return nil, fmt.Errorf("get status for %q: %w", name, err)
	}

	return releaseToInfo(rel), nil
}

func (c *Client) List() ([]ReleaseInfo, error) {
	cfg, err := c.actionConfig()
	if err != nil {
		return nil, err
	}

	list := action.NewList(cfg)
	list.All = true
	releases, err := list.Run()
	if err != nil {
		return nil, fmt.Errorf("list releases: %w", err)
	}

	infos := make([]ReleaseInfo, 0, len(releases))
	for _, rel := range releases {
		infos = append(infos, *releaseToInfo(rel))
	}
	return infos, nil
}

func (c *Client) ReleaseExists(name string) bool {
	_, err := c.Status(name)
	return err == nil
}

func releaseToInfo(rel *release.Release) *ReleaseInfo {
	info := &ReleaseInfo{
		Name:      rel.Name,
		Namespace: rel.Namespace,
		Status:    string(rel.Info.Status),
		Revision:  rel.Version,
	}
	if !rel.Info.LastDeployed.IsZero() {
		info.Updated = rel.Info.LastDeployed.Time
	}
	return info
}
