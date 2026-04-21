// pkg/chart/dependencies.go
package chart

type DependencyInfo struct {
	ChartName     string
	Repository    string
	DefaultValues map[string]interface{}
}

var DependencyRegistry = map[string]DependencyInfo{
	"mysql": {
		ChartName:  "mysql",
		Repository: "https://charts.bitnami.com/bitnami",
		DefaultValues: map[string]interface{}{
			"auth": map[string]interface{}{
				"database": "app",
			},
			"primary": map[string]interface{}{
				"persistence": map[string]interface{}{
					"size": "1Gi",
				},
			},
		},
	},
	"postgresql": {
		ChartName:  "postgresql",
		Repository: "https://charts.bitnami.com/bitnami",
		DefaultValues: map[string]interface{}{
			"auth": map[string]interface{}{
				"database": "app",
			},
			"primary": map[string]interface{}{
				"persistence": map[string]interface{}{
					"size": "1Gi",
				},
			},
		},
	},
	"redis": {
		ChartName:  "redis",
		Repository: "https://charts.bitnami.com/bitnami",
		DefaultValues: map[string]interface{}{
			"architecture": "standalone",
			"master": map[string]interface{}{
				"persistence": map[string]interface{}{
					"size": "1Gi",
				},
			},
		},
	},
	"mongodb": {
		ChartName:  "mongodb",
		Repository: "https://charts.bitnami.com/bitnami",
		DefaultValues: map[string]interface{}{
			"architecture": "standalone",
			"persistence": map[string]interface{}{
				"size": "1Gi",
			},
		},
	},
}
