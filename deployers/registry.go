// Package deployers provides a global registry for deployer implementations
// and includes the built-in shell deployer and external plugin support.
//
// Built-in deployers register themselves in init() functions. External plugins
// are registered at runtime via RegisterPath (from the resolver) or discovered
// automatically by looking for deploy-plugin-<type> executables in PATH and
// the ./plugins/ directory.
package deployers

import (
	"fmt"
	"sync"

	"github.com/smavropoulos/deploy/types"
)

var (
	mu       sync.RWMutex
	registry = map[string]types.Deployer{}
)

// Register adds a deployer to the global registry.
func Register(d types.Deployer) {
	mu.Lock()
	defer mu.Unlock()
	registry[d.Type()] = d
}

// RegisterPath registers an external plugin deployer by name and executable path.
func RegisterPath(name, executablePath string) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = &pluginDeployer{typeName: name, execPath: executablePath}
}

// Get returns a deployer by type name. If no built-in deployer is found,
// it falls back to looking for an external plugin executable.
func Get(typeName string) (types.Deployer, error) {
	mu.RLock()
	d, ok := registry[typeName]
	mu.RUnlock()
	if ok {
		return d, nil
	}

	// Try external plugin
	p := &pluginDeployer{typeName: typeName}
	if path := p.resolveExecutable(); path != "" {
		p.execPath = path
		// Cache in registry for subsequent lookups
		mu.Lock()
		registry[typeName] = p
		mu.Unlock()
		return p, nil
	}

	return nil, fmt.Errorf("unknown deployer type %q (no built-in or plugin found)", typeName)
}

// ExecPath returns the resolved executable path for a deployer, or ""
// for built-in deployers (like "shell") that don't have an external binary.
func ExecPath(typeName string) string {
	mu.RLock()
	d, ok := registry[typeName]
	mu.RUnlock()
	if ok {
		if p, isPlugin := d.(*pluginDeployer); isPlugin {
			return p.execPath
		}
		return ""
	}
	return ""
}
