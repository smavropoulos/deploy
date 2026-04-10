// Package types defines the core data structures shared across the deploy tool.
//
// It contains the Deployer interface that all deployers must implement,
// the YAML schema types (DeploymentFile, DeployStep), and the database
// record types (DeploymentRecord, ConfigEntry).
package types

import (
	"context"
	"time"
)

// Deployer is the interface that all deployers (built-in or plugin) must implement.
// Built-in deployers register themselves via deployers.Register() in an init() function.
// External plugins implement this protocol over JSON stdin/stdout (see deployers/plugin.go).
type Deployer interface {
	// Type returns the deployer's unique identifier (e.g. "shell", "ftp-deploy").
	Type() string

	// Deploy executes the deployment step. It receives the fully resolved
	// environment variables and step configuration, and returns the combined
	// output and any error encountered.
	Deploy(ctx context.Context, step DeployStep, env map[string]string) (output string, err error)
}

// DeploymentFile represents the top-level structure of a deployment YAML file.
//
// Example:
//
//	uses:
//	  - github.com/owner/plugin:v1.0.0
//	env:
//	  VERSION: "1.0.0"
//	secrets:
//	  - API_KEY
//	deploy:
//	  - name: Build
//	    command: ["go build -o app ."]
type DeploymentFile struct {
	Uses    []string          `yaml:"uses,omitempty"`    // Git refs to external plugins (host/owner/repo:tag)
	Env     map[string]string `yaml:"env"`               // Environment variables available to all steps
	Secrets []string          `yaml:"secrets,omitempty"` // Env var names whose values are masked in output
	Deploy  []DeployStep      `yaml:"deploy"`            // Ordered list of deployment steps
}

// DeployStep is a single step inside a deployment pipeline.
type DeployStep struct {
	Name        string            `yaml:"name"`              // Human-readable step name
	Description string            `yaml:"description"`       // Displayed during execution; supports ${{ expr }} and ${VAR}
	Type        string            `yaml:"type"`              // Deployer type (defaults to "shell" if omitted)
	Uses        string            `yaml:"uses,omitempty"`    // Shorthand: infer type from a uses reference
	When        string            `yaml:"when,omitempty"`    // expr-lang boolean condition; step is skipped if false
	Command     []string          `yaml:"command,omitempty"` // Shell commands (used by the built-in shell deployer)
	Config      map[string]string `yaml:"config,omitempty"`  // Arbitrary key-value config passed to the deployer
}

// DeploymentRecord is a row in the SQLite deployments table, tracking each
// step execution with its status, output, and timing information.
type DeploymentRecord struct {
	ID          int64
	Name        string
	Hash        string // SHA-256 of the deployment file contents
	Description string
	FilePath    string
	Status      string // "pending", "running", "success", or "failed"
	StartedAt   time.Time
	FinishedAt  *time.Time // nil if still running
	Output      string
	CreatedAt   time.Time
}

// ConfigEntry is a key-value pair stored in the SQLite config table.
type ConfigEntry struct {
	Key       string
	Value     string
	UpdatedAt time.Time
}
