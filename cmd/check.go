package cmd

import (
	"fmt"
	"strconv"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/smavropoulos/deploy/db"
	"github.com/smavropoulos/deploy/types"
)

func NewCheckCmd(database *db.DB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check [id-or-filter]",
		Short: "Check status of deployments",
		Long: `Check the status of deployments. You can query by:
  - numeric ID        deploy check 42
  - name              deploy check Deploy
  - hash (prefix)     deploy check a1b2c3
  - status            deploy check failed
  - description text  deploy check "version 2026"
  - (no args)         deploy check        (lists all)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var records []types.DeploymentRecord

			if len(args) == 0 {
				r, err := database.QueryDeployments("")
				if err != nil {
					return err
				}
				records = r
			} else if id, err := strconv.ParseInt(args[0], 10, 64); err == nil {
				rec, err := database.GetDeployment(id)
				if err != nil {
					return fmt.Errorf("deployment #%d not found", id)
				}
				records = []types.DeploymentRecord{*rec}
			} else {
				r, err := database.QueryDeployments(args[0])
				if err != nil {
					return err
				}
				records = r
			}

			if len(records) == 0 {
				pterm.Info.Println("No deployments found.")
				return nil
			}

			printDeploymentsTable(records)
			return nil
		},
	}
	return cmd
}

func statusStyle(status string) string {
	switch status {
	case "success":
		return pterm.LightGreen(status)
	case "failed":
		return pterm.LightRed(status)
	case "running":
		return pterm.LightYellow(status)
	default:
		return pterm.Gray(status)
	}
}

func printDeploymentsTable(records []types.DeploymentRecord) {
	data := pterm.TableData{
		{"ID", "Name", "Status", "Description", "Hash", "Started", "Finished"},
	}
	for _, r := range records {
		finished := "-"
		if r.FinishedAt != nil {
			finished = r.FinishedAt.Format("15:04:05")
		}
		hash := r.Hash
		if len(hash) > 8 {
			hash = hash[:8]
		}
		data = append(data, []string{
			fmt.Sprintf("%d", r.ID),
			r.Name,
			statusStyle(r.Status),
			r.Description,
			hash,
			r.StartedAt.Format("15:04:05"),
			finished,
		})
	}
	pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(data).Render()
}
