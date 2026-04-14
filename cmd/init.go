package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/smavropoulos/deploy/db"
)

const deploymentTemplate = `env:
  APP_NAME: %s
  VERSION: "1.0.0"

deploy:
  - name: Build
    description: Building ${{ APP_NAME }} v${{ VERSION }}
    command:
      - echo "Building ${APP_NAME} v${VERSION}..."

  - name: Deploy
    description: Deploying ${{ APP_NAME }}
    command:
      - echo "Deploying ${APP_NAME} v${VERSION}..."
`

func NewInitCmd(database *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize a new deployment project in the current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			dirName := filepath.Base(cwd)
			fileName := dirName + ".deployment.yaml"

			if _, err := os.Stat(fileName); err == nil {
				return fmt.Errorf("%s already exists", fileName)
			}

			content := fmt.Sprintf(deploymentTemplate, dirName)
			if err := os.WriteFile(fileName, []byte(content), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", fileName, err)
			}

			pterm.Success.Printfln("Created %s", fileName)
			return nil
		},
	}
}
