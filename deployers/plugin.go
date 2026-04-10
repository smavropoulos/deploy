package deployers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/smavropoulos/deploy/types"
)

// PluginRequest is the JSON payload sent to a plugin's stdin.
type PluginRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Config      map[string]string `json:"config"`
	Env         map[string]string `json:"env"`
}

// PluginResponse is the JSON payload expected from a plugin's stdout.
type PluginResponse struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}

type pluginDeployer struct {
	typeName string
	execPath string // if set, use this exact path instead of searching
}

func (p *pluginDeployer) Type() string { return p.typeName }

func (p *pluginDeployer) Deploy(ctx context.Context, step types.DeployStep, env map[string]string) (string, error) {
	exe := p.execPath
	if exe == "" {
		exe = p.resolveExecutable()
	}
	if exe == "" {
		return "", fmt.Errorf("plugin executable for %q not found in PATH or plugins directory", p.typeName)
	}

	req := PluginRequest{
		Name:        step.Name,
		Description: step.Description,
		Config:      step.Config,
		Env:         env,
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal plugin request: %w", err)
	}

	c := exec.CommandContext(ctx, exe)
	c.Stdin = strings.NewReader(string(payload))
	c.Env = os.Environ()
	for k, v := range env {
		c.Env = append(c.Env, k+"="+v)
	}

	out, err := c.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("plugin %q failed: %w\n%s", p.typeName, err, string(out))
	}

	var resp PluginResponse
	if jsonErr := json.Unmarshal(out, &resp); jsonErr != nil {
		// If the plugin doesn't return JSON, just treat stdout as output
		return string(out), nil
	}

	if !resp.Success {
		return resp.Output, fmt.Errorf("plugin %q reported error: %s", p.typeName, resp.Error)
	}
	return resp.Output, nil
}

func (p *pluginDeployer) execNames() []string {
	base := "deploy-plugin-" + p.typeName
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	names := []string{
		base + "-" + runtime.GOOS + "-" + runtime.GOARCH + ext,
		base + "-" + runtime.GOOS + ext,
	}
	if ext != "" {
		names = append(names, base+ext)
	}
	names = append(names, base)
	return names
}

// resolveExecutable looks for the plugin in PATH, then in a ./plugins directory
// next to the current working directory. Probes OS/arch-specific variants.
func (p *pluginDeployer) resolveExecutable() string {
	for _, name := range p.execNames() {
		// Check PATH
		if path, err := exec.LookPath(name); err == nil {
			return path
		}

		// Check ./plugins/ relative to cwd
		if cwd, err := os.Getwd(); err == nil {
			candidate := filepath.Join(cwd, "plugins", name)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}

	return ""
}
