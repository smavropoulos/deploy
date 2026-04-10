package deployers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/smavropoulos/deploy/eval"
	"github.com/smavropoulos/deploy/types"
)

func init() {
	Register(&shellDeployer{})
}

// shellDeployer is the built-in deployer that executes shell commands.
// On Windows it uses cmd /C; on Unix it uses sh -c.
// Commands support ${{ expr }} expansion via the eval package.
type shellDeployer struct{}

func (s *shellDeployer) Type() string { return "shell" }

func (s *shellDeployer) Deploy(ctx context.Context, step types.DeployStep, env map[string]string) (string, error) {
	var output strings.Builder
	for _, raw := range step.Command {
		expanded, err := eval.Expand(raw, env)
		if err != nil {
			return output.String(), fmt.Errorf("expand command: %w", err)
		}

		var c *exec.Cmd
		if runtime.GOOS == "windows" {
			c = exec.CommandContext(ctx, "cmd", "/C", expanded)
		} else {
			c = exec.CommandContext(ctx, "sh", "-c", expanded)
		}

		c.Env = os.Environ()
		for k, v := range env {
			c.Env = append(c.Env, k+"="+v)
		}

		out, err := c.CombinedOutput()
		output.Write(out)
		if err != nil {
			return output.String(), fmt.Errorf("%w\n%s", err, string(out))
		}
	}
	return output.String(), nil
}
