// powershell-deploy is an external deployer plugin that executes PowerShell commands.
//
// It runs commands via pwsh (PowerShell Core) in non-interactive mode.
// Commands can be specified via the "command" config key (single command)
// or "commands" (newline-separated list). All commands are joined into
// a single script block.
//
// Config keys:
//
//	command   - A single PowerShell command to execute
//	commands  - Multiple commands separated by newlines
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// PluginRequest matches the JSON payload sent by the deploy tool on stdin.
type PluginRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Config      map[string]string `json:"config"`
	Env         map[string]string `json:"env"`
}

// PluginResponse is the JSON payload returned to the deploy tool on stdout.
type PluginResponse struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}

func main() {
	var req PluginRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		fatal("failed to read request: " + err.Error())
	}

	// Collect commands: either "commands" (newline-separated) or single "command"
	var commands []string
	if cmds, ok := req.Config["commands"]; ok {
		for _, line := range strings.Split(cmds, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				commands = append(commands, line)
			}
		}
	}
	if cmd, ok := req.Config["command"]; ok {
		commands = append(commands, cmd)
	}

	if len(commands) == 0 {
		fatal("no commands provided: set 'command' or 'commands' in config")
	}

	// Expand env vars in commands
	for i, cmd := range commands {
		commands[i] = expandEnv(cmd, req.Env)
	}

	// Join all commands into a single script block
	script := strings.Join(commands, "\n")

	var output strings.Builder
	c := exec.Command("pwsh", "-NoProfile", "-NonInteractive", "-Command", script)

	// Pass env vars
	c.Env = os.Environ()
	for k, v := range req.Env {
		c.Env = append(c.Env, k+"="+v)
	}

	out, err := c.CombinedOutput()
	output.Write(out)

	if err != nil {
		respond(PluginResponse{
			Success: false,
			Output:  output.String(),
			Error:   fmt.Sprintf("pwsh exited with error: %v", err),
		})
		return
	}

	respond(PluginResponse{
		Success: true,
		Output:  output.String(),
	})
}

func expandEnv(s string, env map[string]string) string {
	for k, v := range env {
		s = strings.ReplaceAll(s, "${env."+k+"}", v)
		s = strings.ReplaceAll(s, "${"+k+"}", v)
	}
	return s
}

func respond(resp PluginResponse) {
	json.NewEncoder(os.Stdout).Encode(resp)
}

func fatal(msg string) {
	respond(PluginResponse{
		Success: false,
		Error:   msg,
	})
	os.Exit(1)
}
