package cmd

import (
	"fmt"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/smavropoulos/deploy/db"
)

// NewPluginsCmd creates the "deploy plugins" command for listing registered plugins.
func NewPluginsCmd(database *db.DB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugins",
		Short: "List registered deployer plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			plugins, err := database.ListPlugins()
			if err != nil {
				return err
			}

			if len(plugins) == 0 {
				pterm.Info.Println("No plugins registered yet.")
				pterm.FgGray.Println("Plugins are recorded automatically when used in a deployment.")
				return nil
			}

			data := pterm.TableData{
				{"Name", "Source", "Version", "Exec Path", "Installed", "Last Used"},
			}
			for _, p := range plugins {
				lastUsed := "-"
				if p.LastUsedAt != nil {
					lastUsed = p.LastUsedAt.Format("2006-01-02 15:04:05")
				}
				data = append(data, []string{
					pterm.Bold.Sprint(p.Name),
					p.Source,
					versionOrDash(p.Version),
					p.ExecPath,
					p.InstalledAt.Format("2006-01-02 15:04:05"),
					lastUsed,
				})
			}

			pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(data).Render()
			fmt.Println()
			pterm.FgGray.Printfln("%d plugin(s) registered.", len(plugins))
			return nil
		},
	}
	return cmd
}

func versionOrDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}
