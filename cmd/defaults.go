package cmd

import (
	"bufio"
	"os"
	"strings"
)

var (
	_                      = loadEnvFile(".env")
	defaultRegistry        = envOrFallback("VELA_REGISTRY", "registry.example.com/myteam")
	defaultDomain          = envOrFallback("VELA_DOMAIN", "apps.example.com")
	defaultBaseRegistry    = envOrFallback("VELA_BASE_REGISTRY", "")
	defaultDBImageRegistry = envOrFallback("VELA_DB_IMAGE_REGISTRY", "")
)

func envOrFallback(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadEnvFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
	return true
}
