package eval

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/expr-lang/expr"
)

var exprPattern = regexp.MustCompile(`\$\{\{(.+?)\}\}`)

// Expand processes a string, evaluating any ${{ expr }} blocks using expr-lang.
// The env map is available inside expressions as top-level variables.
// Regular ${VAR} / ${env.VAR} substitutions are also performed.
func Expand(s string, env map[string]string) (string, error) {
	// First do simple env substitution for ${VAR} and ${env.VAR}
	s = expandEnvVars(s, env)

	// Then evaluate ${{ expr }} blocks
	var evalErr error
	result := exprPattern.ReplaceAllStringFunc(s, func(match string) string {
		if evalErr != nil {
			return match
		}
		parts := exprPattern.FindStringSubmatch(match)
		expression := strings.TrimSpace(parts[1])

		val, err := Eval(expression, env)
		if err != nil {
			evalErr = fmt.Errorf("expr %q: %w", expression, err)
			return match
		}
		return fmt.Sprintf("%v", val)
	})
	return result, evalErr
}

// Eval evaluates a single expr-lang expression with the given env as variables.
func Eval(expression string, env map[string]string) (any, error) {
	// Build the environment: all env vars available as top-level identifiers,
	// plus an "env" map for env.KEY access.
	exprEnv := map[string]any{
		"env": toAnyMap(env),
	}
	for k, v := range env {
		exprEnv[k] = v
	}

	program, err := expr.Compile(expression, expr.Env(exprEnv))
	if err != nil {
		return nil, err
	}
	return expr.Run(program, exprEnv)
}

// EvalBool evaluates an expression that should return a boolean.
func EvalBool(expression string, env map[string]string) (bool, error) {
	val, err := Eval(expression, env)
	if err != nil {
		return false, err
	}
	b, ok := val.(bool)
	if !ok {
		return false, fmt.Errorf("expression %q returned %T, expected bool", expression, val)
	}
	return b, nil
}

func expandEnvVars(s string, env map[string]string) string {
	for k, v := range env {
		s = strings.ReplaceAll(s, "${env."+k+"}", v)
		s = strings.ReplaceAll(s, "${"+k+"}", v)
	}
	return s
}

func toAnyMap(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
