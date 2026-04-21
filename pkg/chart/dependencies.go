// pkg/chart/dependencies.go
package chart

type DependencyInfo struct {
	ChartName     string
	Repository    string
	DefaultValues map[string]any
}

var DependencyRegistry = map[string]DependencyInfo{
	"mysql": {
		ChartName:  "mysql",
		Repository: "https://charts.bitnami.com/bitnami",
		DefaultValues: map[string]any{
			"auth": map[string]any{
				"database": "app",
			},
			"primary": map[string]any{
				"persistence": map[string]any{
					"size": "1Gi",
				},
			},
		},
	},
	"postgresql": {
		ChartName:  "postgresql",
		Repository: "https://charts.bitnami.com/bitnami",
		DefaultValues: map[string]any{
			"auth": map[string]any{
				"database": "app",
			},
			"primary": map[string]any{
				"persistence": map[string]any{
					"size": "1Gi",
				},
			},
		},
	},
	"redis": {
		ChartName:  "redis",
		Repository: "https://charts.bitnami.com/bitnami",
		DefaultValues: map[string]any{
			"architecture": "standalone",
			"master": map[string]any{
				"persistence": map[string]any{
					"size": "1Gi",
				},
			},
		},
	},
	"mongodb": {
		ChartName:  "mongodb",
		Repository: "https://charts.bitnami.com/bitnami",
		DefaultValues: map[string]any{
			"architecture": "standalone",
			"persistence": map[string]any{
				"size": "1Gi",
			},
		},
	},
}
