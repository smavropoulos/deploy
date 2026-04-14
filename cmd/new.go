package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/smavropoulos/deploy/db"
)

func NewNewCmd(database *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "new <name>",
		Short: "Create a new deployment project in a subfolder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if err := os.MkdirAll(name, 0o755); err != nil {
				return fmt.Errorf("create directory %s: %w", name, err)
			}

			fileName := filepath.Join(name, name+".deployment.yaml")

			if _, err := os.Stat(fileName); err == nil {
				return fmt.Errorf("%s already exists", fileName)
			}

			content := fmt.Sprintf(deploymentTemplate, name)
			if err := os.WriteFile(fileName, []byte(content), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", fileName, err)
			}

			pterm.Success.Printfln("Created %s", fileName)
			return nil
		},
	}
}
