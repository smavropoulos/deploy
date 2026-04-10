// Package cmd implements the CLI commands for the deploy tool.
package cmd

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/smavropoulos/deploy/db"
	"github.com/smavropoulos/deploy/deployers"
	"github.com/smavropoulos/deploy/eval"
	"github.com/smavropoulos/deploy/redact"
	"github.com/smavropoulos/deploy/resolver"
	"github.com/smavropoulos/deploy/types"
)

func NewRunCmd(database *db.DB) *cobra.Command {
	var filePath string
	var envOverrides []string

	cmd := &cobra.Command{
		Use:           "run",
		Short:         "Run a deployment from a YAML file",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}

			var df types.DeploymentFile
			if err := yaml.Unmarshal(data, &df); err != nil {
				return fmt.Errorf("parse yaml: %w", err)
			}

			// Merge env: file defaults + overrides from -e flags
			env := make(map[string]string)
			for k, v := range df.Env {
				env[k] = resolveDefault(v)
			}
			for _, ov := range envOverrides {
				parts := strings.SplitN(ov, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid env override %q (expected key=value)", ov)
				}
				env[parts[0]] = parts[1]
			}

			// Resolve uses: fetch & register external deployers
			if len(df.Uses) > 0 {
				cacheDir := filepath.Join(".", ".deploy", "plugins")
				if err := resolver.ResolveAll(df.Uses, cacheDir); err != nil {
					return err
				}
				pterm.Println()
			}

			hash := fmt.Sprintf("%x", sha256.Sum256(data))

			// Build redactor from secrets list
			rd := redact.New(env, df.Secrets)

			total := len(df.Deploy)

			pterm.Println()

			for i, step := range df.Deploy {
				prefix := pterm.LightBlue(fmt.Sprintf("[%d/%d]", i+1, total))
				// Evaluate when condition
				if step.When != "" {
					ok, err := eval.EvalBool(step.When, env)
					if err != nil {
						return fmt.Errorf("step %q when condition: %w", step.Name, err)
					}
					if !ok {
						pterm.Warning.Printfln("%s %s skipped (when: %s)", prefix, step.Name, step.When)
						continue
					}
				}

				// Determine deployer type
				typeName := step.Type
				if typeName == "" && step.Uses != "" {
					// Resolve inline uses on the step (should already be in top-level uses)
					typeName = resolveTypeName(step.Uses)
				}
				if typeName == "" {
					typeName = "shell"
				}

				d, err := deployers.Get(typeName)
				if err != nil {
					return err
				}

				desc, err := eval.Expand(step.Description, env)
				if err != nil {
					return fmt.Errorf("step %q description: %w", step.Name, err)
				}
				desc = rd.Redact(desc)
				id, err := database.InsertDeployment(step.Name, hash, desc, filePath)
				if err != nil {
					return fmt.Errorf("record deployment: %w", err)
				}

				spinner, _ := pterm.DefaultSpinner.
					WithText(fmt.Sprintf("%s %s (%s) — %s", prefix, pterm.Bold.Sprint(step.Name), pterm.Gray(typeName), desc)).
					Start()

				output, stepErr := d.Deploy(context.Background(), step, env)
				output = rd.Redact(output)

				status := "success"
				if stepErr != nil {
					status = "failed"
				}
				if err := database.UpdateDeploymentStatus(id, status, output); err != nil {
					return fmt.Errorf("update deployment status: %w", err)
				}
				if stepErr != nil {
					spinner.Fail(fmt.Sprintf("%s %s — FAILED", prefix, step.Name))
					if output != "" {
						pterm.DefaultBox.WithTitle("Output").Println(strings.TrimSpace(output))
					}
					return fmt.Errorf("step %q failed: %s", step.Name, rd.Redact(stepErr.Error()))
				}
				spinner.Success(fmt.Sprintf("%s %s", prefix, step.Name))
				if output != "" {
					pterm.FgGray.Println(strings.TrimSpace(output))
				}
			}

			pterm.Println()
			pterm.Success.Printfln("All %d steps completed successfully.", total)
			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to deployment YAML file")
	cmd.MarkFlagRequired("file")
	cmd.Flags().StringArrayVarP(&envOverrides, "env", "e", nil, "Environment overrides (key=value)")

	return cmd
}

// resolveDefault handles ${VAR:-default} syntax, falling back to OS env.
func resolveDefault(val string) string {
	re := regexp.MustCompile(`\$\{(\w+):-"?([^"]*)"?\}`)
	return re.ReplaceAllStringFunc(val, func(match string) string {
		parts := re.FindStringSubmatch(match)
		envVal := os.Getenv(parts[1])
		if envVal != "" {
			return envVal
		}
		return parts[2]
	})
}

// resolveTypeName extracts the deployer name from a uses ref.
// e.g. "github.com/smavropoulos/ftp-deploy:1.0.0" -> "ftp-deploy"
func resolveTypeName(ref string) string {
	ref = strings.TrimPrefix(ref, "https://")
	// Strip tag
	if idx := strings.LastIndex(ref, ":"); idx != -1 {
		ref = ref[:idx]
	}
	// Take last path segment as the deployer name
	if idx := strings.LastIndex(ref, "/"); idx != -1 {
		return ref[idx+1:]
	}
	return ref
}
