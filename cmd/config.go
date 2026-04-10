package cmd

import (
	"fmt"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/smavropoulos/deploy/db"
)

func NewConfigCmd(database *db.DB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage key-value configuration",
	}

	cmd.AddCommand(newConfigGetCmd(database))
	cmd.AddCommand(newConfigSetCmd(database))
	cmd.AddCommand(newConfigDeleteCmd(database))
	cmd.AddCommand(newConfigListCmd(database))

	return cmd
}

func newConfigGetCmd(database *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, err := database.GetConfig(args[0])
			if err != nil {
				return fmt.Errorf("config key %q not found", args[0])
			}
			pterm.Info.Printfln("%s = %s", pterm.Bold.Sprint(args[0]), entry.Value)
			return nil
		},
	}
}

func newConfigSetCmd(database *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := database.SetConfig(args[0], args[1]); err != nil {
				return err
			}
			pterm.Success.Printfln("%s = %s", pterm.Bold.Sprint(args[0]), args[1])
			return nil
		},
	}
}

func newConfigDeleteCmd(database *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <key>",
		Short: "Delete a config key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return database.DeleteConfig(args[0])
		},
	}
}

func newConfigListCmd(database *db.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all config entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := database.ListConfig()
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				pterm.Info.Println("No config entries.")
				return nil
			}
			data := pterm.TableData{{"Key", "Value", "Updated"}}
			for _, e := range entries {
				data = append(data, []string{e.Key, e.Value, e.UpdatedAt.Format("2006-01-02 15:04:05")})
			}
			pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(data).Render()
			return nil
		},
	}
}
