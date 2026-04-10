// Package resolver handles fetching and caching external deployer plugins
// referenced by the "uses" field in deployment YAML files.
//
// Plugins are specified as Git references (e.g. "github.com/owner/repo:v1.0.0").
// On first use, the repository is cloned into .deploy/plugins/ and cached.
// Each plugin repo must contain a deploy-plugin.yaml manifest that declares
// the plugin's name, description, and entrypoint binary.
//
// The entrypoint is resolved in an OS/architecture-aware manner:
//
//	<base>-<os>-<arch>[.exe]  →  <base>-<os>[.exe]  →  <base>[.exe]  →  <base>
package resolver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pterm/pterm"
	"gopkg.in/yaml.v3"

	"github.com/smavropoulos/deploy/db"
	"github.com/smavropoulos/deploy/deployers"
)

// PluginManifest represents the deploy-plugin.yaml file found in a plugin repository.
type PluginManifest struct {
	Name        string `yaml:"name"`        // Deployer type name registered in the registry (e.g. "ftp-deploy")
	Description string `yaml:"description"` // Human-readable description of the plugin
	Version     string `yaml:"version"`     // Semantic version (e.g. "1.0.0")
	Entrypoint  string `yaml:"entrypoint"`  // Base name of the executable (without .exe); resolved per OS/arch
}

// ResolveAll fetches/caches all plugins declared in uses and registers them.
// If database is non-nil, resolved plugins are recorded in the plugins table.
func ResolveAll(uses []string, cacheDir string, database *db.DB) error {
	if len(uses) == 0 {
		return nil
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("create plugin cache dir: %w", err)
	}

	for _, ref := range uses {
		if err := resolveOne(ref, cacheDir, database); err != nil {
			return fmt.Errorf("resolve %q: %w", ref, err)
		}
	}
	return nil
}

// resolveOne parses a ref like "github.com/owner/repo:tag", clones or
// updates the repo, reads deploy-plugin.yaml, and registers the deployer.
func resolveOne(ref, cacheDir string, database *db.DB) error {
	repoURL, tag, err := parseRef(ref)
	if err != nil {
		return err
	}

	// Cache key: flatten the URL into a directory name + tag
	safeName := strings.ReplaceAll(repoURL, "/", "_")
	safeName = strings.ReplaceAll(safeName, ":", "_")
	pluginDir := filepath.Join(cacheDir, fmt.Sprintf("%s@%s", safeName, tag))

	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		// Clone at the specified tag
		cloneURL := "https://" + repoURL + ".git"
		pterm.Info.Printfln("Fetching plugin %s@%s", repoURL, tag)

		args := []string{"clone", "--depth", "1", "--branch", tag, cloneURL, pluginDir}
		cmd := exec.Command("git", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git clone %s: %w", cloneURL, err)
		}
	} else {
		pterm.FgGray.Printfln("Using cached plugin %s@%s", repoURL, tag)
	}

	// Read manifest
	manifest, err := readManifest(pluginDir)
	if err != nil {
		return err
	}

	// Resolve entrypoint path for the current OS/arch
	entrypoint, err := resolveEntrypoint(pluginDir, manifest.Entrypoint)
	if err != nil {
		return fmt.Errorf("plugin %s: %w", ref, err)
	}

	// Register as an external plugin deployer pointing at the resolved executable
	deployers.RegisterPath(manifest.Name, entrypoint)

	// Record the plugin in the database for tracking
	if database != nil {
		source := "git:" + repoURL
		version := manifest.Version
		if version == "" {
			version = tag
		}
		database.UpsertPlugin(manifest.Name, source, version, entrypoint)
	}

	pterm.Success.Printfln("Registered deployer %q from %s@%s", manifest.Name, repoURL, tag)
	return nil
}

// parseRef parses "github.com/owner/repo:tag" into ("github.com/owner/repo", "tag").
func parseRef(ref string) (string, string, error) {
	// Strip https:// prefix if present
	ref = strings.TrimPrefix(ref, "https://")

	idx := strings.LastIndex(ref, ":")
	if idx == -1 {
		return "", "", fmt.Errorf("invalid uses ref %q: expected format host/owner/repo:tag", ref)
	}
	repoURL := ref[:idx]
	tag := ref[idx+1:]
	if repoURL == "" || tag == "" {
		return "", "", fmt.Errorf("invalid uses ref %q: empty repo or tag", ref)
	}
	return repoURL, tag, nil
}

// readManifest reads and validates the deploy-plugin.yaml file in a plugin directory.
func readManifest(pluginDir string) (*PluginManifest, error) {
	data, err := os.ReadFile(filepath.Join(pluginDir, "deploy-plugin.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read deploy-plugin.yaml: %w (plugin must contain a deploy-plugin.yaml manifest)", err)
	}
	var m PluginManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse deploy-plugin.yaml: %w", err)
	}
	if m.Name == "" {
		return nil, fmt.Errorf("deploy-plugin.yaml missing required 'name' field")
	}
	if m.Entrypoint == "" {
		return nil, fmt.Errorf("deploy-plugin.yaml missing required 'entrypoint' field")
	}
	return &m, nil
}

// resolveEntrypoint finds the right binary for the current OS/arch.
// Given a base name like "deploy-plugin-ftp", it probes in order:
//  1. <base>-<os>-<arch>[.exe]   (e.g. deploy-plugin-ftp-windows-amd64.exe)
//  2. <base>-<os>[.exe]          (e.g. deploy-plugin-ftp-windows.exe)
//  3. <base>[.exe]               (e.g. deploy-plugin-ftp.exe)
//  4. <base>                     (e.g. deploy-plugin-ftp)
func resolveEntrypoint(pluginDir, baseName string) (string, error) {
	// Strip any extension the manifest author may have included
	baseName = strings.TrimSuffix(baseName, ".exe")

	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	candidates := []string{
		baseName + "-" + runtime.GOOS + "-" + runtime.GOARCH + ext,
		baseName + "-" + runtime.GOOS + ext,
	}
	if ext != "" {
		candidates = append(candidates, baseName+ext)
	}
	candidates = append(candidates, baseName)

	for _, name := range candidates {
		path := filepath.Join(pluginDir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf(
		"no entrypoint found for %s on %s/%s (tried: %s)",
		baseName, runtime.GOOS, runtime.GOARCH, strings.Join(candidates, ", "),
	)
}
