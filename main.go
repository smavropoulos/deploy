package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/smavropoulos/deploy/cmd"
	"github.com/smavropoulos/deploy/db"
	_ "github.com/smavropoulos/deploy/deployers" // register built-in deployers
)

func main() {

	dbDir := filepath.Join(".\\", ".deploy")
	os.MkdirAll(dbDir, 0o755)

	database, err := db.Open(filepath.Join(dbDir, "deploy.db"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer database.Close()

	pterm.Println()

	rootCmd := &cobra.Command{
		Use:           "deploy",
		Short:         "A deployment orchestration tool",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.AddCommand(cmd.NewRunCmd(database))
	rootCmd.AddCommand(cmd.NewConfigCmd(database))
	rootCmd.AddCommand(cmd.NewCheckCmd(database))

	if err := rootCmd.Execute(); err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}
}
