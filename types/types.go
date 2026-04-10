package types

import (
	"context"
	"time"
)

// Deployer is the interface that all deployers (built-in or third-party) must implement.
type Deployer interface {
	// Type returns the deployer's identifier used in YAML (e.g. "shell", "s3", "ftp").
	Type() string
	// Deploy executes the deployment step. It receives the resolved env and step config,
	// and returns the combined output and any error.
	Deploy(ctx context.Context, step DeployStep, env map[string]string) (output string, err error)
}

// DeploymentFile represents the parsed YAML deployment file.
type DeploymentFile struct {
	Uses   []string          `yaml:"uses,omitempty"` // e.g. "github.com/smavropoulos/ftp-deploy:1.0.0"
	Env    map[string]string `yaml:"env"`
	Deploy []DeployStep      `yaml:"deploy"`
}

// DeployStep is a single step inside a deployment file.
type DeployStep struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Type        string            `yaml:"type"`              // deployer type, defaults to "shell"
	Uses        string            `yaml:"uses,omitempty"`    // shorthand to set type from a uses ref
	When        string            `yaml:"when,omitempty"`    // expr-lang condition; step runs only if true (or empty)
	Command     []string          `yaml:"command,omitempty"` // used by shell deployer
	Config      map[string]string `yaml:"config,omitempty"`  // free-form config for any deployer
}

// DeploymentRecord is a row in the deployments table tracking each run.
type DeploymentRecord struct {
	ID          int64
	Name        string
	Hash        string
	Description string
	FilePath    string
	Status      string // pending, running, success, failed
	StartedAt   time.Time
	FinishedAt  *time.Time
	Output      string
	CreatedAt   time.Time
}

// ConfigEntry is a key-value pair stored in the config table.
type ConfigEntry struct {
	Key       string
	Value     string
	UpdatedAt time.Time
}
