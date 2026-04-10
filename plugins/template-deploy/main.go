// template-deploy is a starter template for building a deploy plugin.
//
// Copy this directory, rename it, and modify it to create your own deployer.
// The deploy tool communicates with plugins over a simple JSON protocol:
//
//  1. deploy sends a PluginRequest as JSON to your plugin's stdin.
//  2. Your plugin does its work (upload files, call APIs, run scripts, etc.).
//  3. Your plugin writes a PluginResponse as JSON to stdout.
//
// That's it! No SDK, no dependencies — just stdin/stdout JSON.
//
// ──────────────────────────────────────────────────────────────────────
// HOW TO USE THIS TEMPLATE
// ──────────────────────────────────────────────────────────────────────
//
//  1. Copy this folder:
//     cp -r plugins/template-deploy plugins/my-deploy
//
//  2. Update go.mod:
//     cd plugins/my-deploy
//     go mod init github.com/yourname/my-deploy
//
//  3. Update deploy-plugin.yaml:
//     name: my-deploy
//     description: What your plugin does.
//     entrypoint: deploy-plugin-my-deploy
//
//  4. Edit main.go — replace the example logic in run() with your own.
//
//  5. Build:
//     go build -o deploy-plugin-my-deploy.exe .
//
//  6. Place the binary in ./plugins/ next to deploy.exe, or on your PATH.
//
//  7. Use it in a deployment YAML:
//     deploy:
//     - name: My Step
//     type: my-deploy
//     config:
//     my_key: my_value
//
// ──────────────────────────────────────────────────────────────────────
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ---------------------------------------------------------------------------
// Protocol types — these must match the deploy tool's plugin protocol.
// Copy them as-is into your plugin. No external dependency needed.
// ---------------------------------------------------------------------------

// PluginRequest is the JSON payload your plugin receives on stdin.
//
// Fields:
//   - Name:        The step name from the YAML file.
//   - Description: The step description (already expanded with expressions).
//   - Config:      Key-value pairs from the step's `config:` block.
//     This is where your plugin reads its settings.
//   - Env:         All environment variables from the deployment file,
//     merged with any -e overrides from the command line.
type PluginRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Config      map[string]string `json:"config"`
	Env         map[string]string `json:"env"`
}

// PluginResponse is the JSON payload your plugin writes to stdout.
//
// Fields:
//   - Success: true if the step succeeded, false otherwise.
//   - Output:  Human-readable output displayed to the user.
//     Keep it concise — deploy shows this in the terminal.
//   - Error:   Error message if Success is false. Displayed in red.
type PluginResponse struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}

// ---------------------------------------------------------------------------
// Main entry point
// ---------------------------------------------------------------------------

func main() {
	// Step 1: Read the request from stdin.
	// The deploy tool sends a single JSON object.
	var req PluginRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		fatal("failed to read request from stdin: " + err.Error())
	}

	// Step 2: (Optional) Expand environment variable placeholders in config values.
	// Config values may contain ${VAR} or ${env.VAR} placeholders that
	// haven't been resolved yet. This helper replaces them.
	for k, v := range req.Config {
		req.Config[k] = expandEnv(v, req.Env)
	}

	// Step 3: Validate required config keys.
	// Fail early with a clear message if something is missing.
	message := req.Config["message"]
	if message == "" {
		fatal("config key 'message' is required")
	}

	// Step 4: Do the actual work.
	// Replace this with your deployment logic — upload files, call APIs,
	// run commands, whatever your plugin does.
	output, err := run(req, message)
	if err != nil {
		// Return a failure response. The deploy tool will mark this step
		// as failed, display the error, and stop the pipeline.
		respond(PluginResponse{
			Success: false,
			Output:  output,
			Error:   err.Error(),
		})
		return
	}

	// Step 5: Return success.
	// The output string is displayed to the user in the terminal.
	respond(PluginResponse{
		Success: true,
		Output:  output,
	})
}

// ---------------------------------------------------------------------------
// Your plugin logic goes here
// ---------------------------------------------------------------------------

// run is where you implement your deployer's core functionality.
// This template just echoes the config back — replace it with real logic.
func run(req PluginRequest, message string) (string, error) {
	var out strings.Builder

	// Example: print the message and some context.
	out.WriteString(fmt.Sprintf("Plugin received step: %s\n", req.Name))
	out.WriteString(fmt.Sprintf("Message: %s\n", message))

	// Example: read an optional config key with a default.
	repeat := req.Config["repeat"]
	if repeat == "" {
		repeat = "1"
	}

	// Example: access environment variables.
	if version, ok := req.Env["VERSION"]; ok {
		out.WriteString(fmt.Sprintf("VERSION from env: %s\n", version))
	}

	// Example: list all config keys (useful for debugging).
	out.WriteString("Config keys received:\n")
	for k, v := range req.Config {
		out.WriteString(fmt.Sprintf("  %s = %s\n", k, v))
	}

	// TODO: Replace the above with your actual deployment logic.
	// For example:
	//   - Upload files to a cloud provider
	//   - Call a REST API
	//   - Run database migrations
	//   - Send notifications (Slack, Teams, email)
	//   - Trigger a CI/CD pipeline
	//   - SSH into a server and run commands

	return out.String(), nil
}

// ---------------------------------------------------------------------------
// Helper functions — copy these into your plugin as-is.
// ---------------------------------------------------------------------------

// expandEnv replaces ${KEY} and ${env.KEY} placeholders with values from
// the env map. This handles the variable syntax used in deployment YAML files.
func expandEnv(s string, env map[string]string) string {
	for k, v := range env {
		s = strings.ReplaceAll(s, "${env."+k+"}", v)
		s = strings.ReplaceAll(s, "${"+k+"}", v)
	}
	return s
}

// respond writes a JSON PluginResponse to stdout. The deploy tool reads
// exactly one JSON object from the plugin's stdout.
//
// IMPORTANT: Do not write anything else to stdout (no fmt.Println, no log).
// Use stderr for debug logging if needed: fmt.Fprintln(os.Stderr, "debug")
func respond(resp PluginResponse) {
	json.NewEncoder(os.Stdout).Encode(resp)
}

// fatal sends an error response and exits with code 1.
// Use this for unrecoverable errors during initialization (e.g. bad input).
func fatal(msg string) {
	respond(PluginResponse{
		Success: false,
		Error:   msg,
	})
	os.Exit(1)
}
