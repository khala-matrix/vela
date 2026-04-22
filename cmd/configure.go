package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var selfUpdate bool

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Manage vela settings and updates",
	RunE:  runConfigure,
}

func init() {
	configureCmd.Flags().BoolVar(&selfUpdate, "self-update", false, "update vela to the latest version")
}

func runConfigure(cmd *cobra.Command, args []string) error {
	if selfUpdate {
		return runSelfUpdate(cmd)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "vela configure")
	fmt.Fprintln(cmd.OutOrStdout(), "No configurable settings yet.")
	fmt.Fprintln(cmd.OutOrStdout(), "\nUse --self-update to update vela to the latest version.")
	return nil
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

const repoAPI = "https://api.github.com/repos/khala-matrix/vela/releases/latest"

func runSelfUpdate(cmd *cobra.Command) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Current version: %s\n", Version)
	fmt.Fprintln(cmd.OutOrStdout(), "Checking for updates...")

	resp, err := http.Get(repoAPI)
	if err != nil {
		return fmt.Errorf("check latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("parse release: %w", err)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(Version, "v")
	if latest == current {
		fmt.Fprintln(cmd.OutOrStdout(), "Already up to date.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "New version available: %s → %s\n", Version, release.TagName)

	assetName := fmt.Sprintf("vela_%s_%s", runtime.GOOS, runtime.GOARCH)
	checksumName := "checksums.txt"

	var assetURL, checksumURL string
	for _, a := range release.Assets {
		if a.Name == assetName {
			assetURL = a.BrowserDownloadURL
		}
		if a.Name == checksumName {
			checksumURL = a.BrowserDownloadURL
		}
	}

	if assetURL == "" {
		return fmt.Errorf("no binary found for %s/%s in release %s", runtime.GOOS, runtime.GOARCH, release.TagName)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Downloading %s...\n", assetName)
	binData, err := downloadFile(assetURL)
	if err != nil {
		return fmt.Errorf("download binary: %w", err)
	}

	if checksumURL != "" {
		fmt.Fprintln(cmd.OutOrStdout(), "Verifying checksum...")
		checksumData, err := downloadFile(checksumURL)
		if err != nil {
			return fmt.Errorf("download checksums: %w", err)
		}
		if err := verifyFileChecksum(binData, checksumData, assetName); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable path: %w", err)
	}

	tmpFile := self + ".new"
	if err := os.WriteFile(tmpFile, binData, 0755); err != nil {
		return fmt.Errorf("write new binary: %w", err)
	}

	if err := os.Rename(tmpFile, self); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("replace binary: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Updated to %s successfully.\n", release.TagName)
	return nil
}

func downloadFile(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func verifyFileChecksum(data []byte, checksumFile []byte, name string) error {
	hash := sha256.Sum256(data)
	got := hex.EncodeToString(hash[:])

	for _, line := range strings.Split(string(checksumFile), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == name {
			if parts[0] == got {
				return nil
			}
			return fmt.Errorf("expected %s, got %s", parts[0], got)
		}
	}
	return fmt.Errorf("no checksum found for %s", name)
}
