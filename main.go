// Package main is the entry point for the deploy CLI tool.
//
// deploy is a deployment orchestration tool that executes multi-step
// deployment pipelines defined in YAML files. It supports built-in
// deployers (shell), external plugins, expr-lang expressions, secret
// redaction, and persistent deployment tracking via SQLite.
//
// Usage:
//
//	deploy run -f deployment.yaml          # execute a deployment
//	deploy check [id-or-filter]            # query deployment history
//	deploy config get|set|delete|list ...  # manage key-value config
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/smavropoulos/deploy/cmd"
	"github.com/smavropoulos/deploy/db"
	_ "github.com/smavropoulos/deploy/deployers" // register built-in deployers via init()
)

// dataDir is the local directory used for the SQLite database and plugin cache.
const dataDir = ".deploy"

func main() {
	// Ensure the data directory exists.
	dbDir := filepath.Join(".", dataDir)
	os.MkdirAll(dbDir, 0o755)

	// Open (or create) the SQLite database.
	database, err := db.Open(filepath.Join(dbDir, "deploy.db"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer database.Close()

	pterm.Println()

	// Build the CLI command tree.
	rootCmd := &cobra.Command{
		Use:           "deploy",
		Short:         "A deployment orchestration tool",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.AddCommand(cmd.NewInitCmd(database))
	rootCmd.AddCommand(cmd.NewNewCmd(database))
	rootCmd.AddCommand(cmd.NewRunCmd(database))
	rootCmd.AddCommand(cmd.NewConfigCmd(database))
	rootCmd.AddCommand(cmd.NewCheckCmd(database))
	rootCmd.AddCommand(cmd.NewPluginsCmd(database))

	if err := rootCmd.Execute(); err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}
}
