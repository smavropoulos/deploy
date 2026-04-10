# deploy

A deployment orchestration CLI tool that executes multi-step pipelines defined in YAML files.

## Features

- **Multi-step pipelines** — Define ordered deployment steps in a single YAML file
- **Built-in shell deployer** — Run shell commands (`cmd` on Windows, `sh` on Unix)
- **Plugin system** — Extend with external deployers via a simple JSON stdin/stdout protocol
- **Expression engine** — Use `${{ expr }}` blocks powered by [expr-lang](https://github.com/expr-lang/expr)
- **Conditional steps** — Skip steps with `when:` conditions evaluated at runtime
- **Secret redaction** — Mask sensitive values in all output with `secrets:`
- **Deployment tracking** — Every step is recorded in a local SQLite database
- **Rich terminal output** — Spinners, colored status, and tables via [pterm](https://github.com/pterm/pterm)
- **Git-based plugin resolution** — Fetch plugins from Git repositories with `uses:`

## Installation

```bash
go install github.com/smavropoulos/deploy@latest
```

Or build from source:

```bash
git clone https://github.com/smavropoulos/deploy.git
cd deploy
go build -o deploy.exe .
```

## Quick Start

Create a file called `my-deploy.yaml`:

```yaml
env:
  APP_NAME: myapp
  VERSION: "1.0.0"

deploy:
  - name: Build
    description: Building ${{ APP_NAME }} v${{ VERSION }}
    command:
      - go build -o ${APP_NAME}.exe .

  - name: Test
    description: Running tests
    command:
      - go test ./...

  - name: Report
    description: Build complete
    command:
      - echo "Successfully built ${APP_NAME} version ${VERSION}"
```

Run it:

```bash
deploy run -f my-deploy.yaml
```

Override environment variables:

```bash
deploy run -f my-deploy.yaml -e VERSION=2.0.0
```

## Commands

### `deploy run`

Execute a deployment pipeline from a YAML file.

```bash
deploy run -f <file.yaml> [-e KEY=VALUE ...]
```

| Flag | Description |
|------|-------------|
| `-f, --file` | Path to the deployment YAML file (required) |
| `-e, --env` | Environment variable overrides (repeatable) |

### `deploy check`

Query deployment history stored in the local SQLite database.

```bash
deploy check              # list all deployments
deploy check 42           # look up by ID
deploy check failed       # filter by status
deploy check "my step"    # search by name or description
```

### `deploy config`

Manage persistent key-value configuration.

```bash
deploy config set my-key my-value
deploy config get my-key
deploy config delete my-key
deploy config list
```

## Deployment YAML Reference

```yaml
# Optional: external plugin references (fetched from Git on first run)
uses:
  - github.com/owner/ftp-deploy:v1.0.0

# Environment variables available to all steps
env:
  VERSION: ${VERSION:-"1.0.0"}     # supports ${VAR:-default} syntax
  API_KEY: ${API_KEY:-""}

# Env var names whose values should be masked in output
secrets:
  - API_KEY

# Ordered deployment steps
deploy:
  - name: Build
    description: Building version ${{ VERSION }}
    # type defaults to "shell" if omitted
    command:
      - go build -o app .

  - name: Deploy
    description: Deploying to server
    type: ftp-deploy                   # use an external plugin
    when: VERSION startsWith '1'       # expr-lang condition
    config:
      host: ftp.example.com
      username: "${FTP_USER}"
      password: "${FTP_PASS}"
      local_path: ./app
      remote_path: /releases/
```

### Expression Blocks

Use `${{ expr }}` anywhere in descriptions, commands, or config values. All env variables are available as top-level identifiers and under `env.KEY`:

```yaml
description: "Deploying ${{ APP_NAME }} (version ${{ env.VERSION }})"
when: |
  VERSION startsWith '2' && APP_NAME != 'test'
```

### Secret Redaction

List env var names under `secrets:` to have their values replaced with `***` in all output:

```yaml
env:
  DB_PASS: ${DB_PASS:-""}
secrets:
  - DB_PASS
```

## Creating a Plugin

Plugins are standalone executables that communicate with `deploy` over JSON via stdin/stdout.

### Plugin Protocol

1. `deploy` sends a **PluginRequest** as JSON to the plugin's **stdin**
2. The plugin executes its logic
3. The plugin writes a **PluginResponse** as JSON to **stdout**

**PluginRequest:**

```json
{
  "name": "Upload Files",
  "description": "Uploading build artifacts",
  "config": {
    "host": "ftp.example.com",
    "path": "/releases/"
  },
  "env": {
    "VERSION": "1.0.0",
    "API_KEY": "secret123"
  }
}
```

**PluginResponse:**

```json
{
  "success": true,
  "output": "Uploaded 3 files to /releases/",
  "error": ""
}
```

### Step-by-Step: Build a "Hello" Plugin

This example creates a minimal plugin called `hello-deploy` that prints a greeting.

#### 1. Create the project

```bash
mkdir hello-deploy && cd hello-deploy
go mod init github.com/yourname/hello-deploy
```

#### 2. Write `main.go`

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"
)

// PluginRequest is the JSON payload received from deploy on stdin.
type PluginRequest struct {
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Config      map[string]string `json:"config"`
    Env         map[string]string `json:"env"`
}

// PluginResponse is the JSON payload sent back to deploy on stdout.
type PluginResponse struct {
    Success bool   `json:"success"`
    Output  string `json:"output"`
    Error   string `json:"error,omitempty"`
}

func main() {
    // 1. Read the request from stdin.
    var req PluginRequest
    if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
        respond(PluginResponse{Success: false, Error: "bad request: " + err.Error()})
        os.Exit(1)
    }

    // 2. Read config values.
    greeting := req.Config["greeting"]
    if greeting == "" {
        greeting = "Hello"
    }
    target := req.Config["target"]
    if target == "" {
        target = "World"
    }

    // 3. Do the work.
    message := fmt.Sprintf("%s, %s! (from plugin, version %s)", greeting, target, req.Env["VERSION"])

    // 4. Return the result on stdout.
    respond(PluginResponse{
        Success: true,
        Output:  message,
    })
}

func respond(resp PluginResponse) {
    json.NewEncoder(os.Stdout).Encode(resp)
}
```

#### 3. Create `deploy-plugin.yaml`

This manifest tells the resolver about your plugin:

```yaml
name: hello-deploy
description: A demo plugin that prints a greeting message.
entrypoint: deploy-plugin-hello-deploy
```

The `name` is the deployer type used in YAML (`type: hello-deploy`).
The `entrypoint` is the base name of the binary — the tool probes OS/arch variants automatically:

| Probe order | Example (Windows amd64) |
|---|---|
| `<base>-<os>-<arch>.exe` | `deploy-plugin-hello-deploy-windows-amd64.exe` |
| `<base>-<os>.exe` | `deploy-plugin-hello-deploy-windows.exe` |
| `<base>.exe` | `deploy-plugin-hello-deploy.exe` |
| `<base>` | `deploy-plugin-hello-deploy` |

#### 4. Build and install

```bash
# Build the plugin binary
go build -o deploy-plugin-hello-deploy.exe .

# Option A: Place it in ./plugins/ next to the deploy binary
cp deploy-plugin-hello-deploy.exe /path/to/project/plugins/

# Option B: Place it on your PATH
cp deploy-plugin-hello-deploy.exe /usr/local/bin/
```

#### 5. Use it in a deployment YAML

```yaml
env:
  VERSION: "1.0.0"

deploy:
  - name: Greet
    description: Saying hello
    type: hello-deploy
    config:
      greeting: "Hey"
      target: "Deploy Tool"
```

Run it:

```bash
deploy run -f hello.yaml
```

Output:

```
 ✓  [1/1] Greet
Hey, Deploy Tool! (from plugin, version 1.0.0)

 SUCCESS  All 1 steps completed successfully.
```

### Distributing via Git

To make your plugin available via `uses:`, push it to a Git repository with:

```
my-plugin/
├── deploy-plugin.yaml              # manifest (required)
├── deploy-plugin-my-plugin.exe     # Windows binary
├── deploy-plugin-my-plugin         # Linux/macOS binary
└── ...
```

Users can then reference it:

```yaml
uses:
  - github.com/yourname/my-plugin:v1.0.0

deploy:
  - name: My Step
    type: my-plugin
    config:
      key: value
```

On first run, `deploy` will clone the repo, read the manifest, and register the plugin automatically. Subsequent runs use the cached copy.

## Project Structure

```
deploy/
├── main.go                 # CLI entry point
├── types/
│   └── types.go            # Shared data types and Deployer interface
├── db/
│   └── db.go               # SQLite persistence layer
├── cmd/
│   ├── run.go              # deploy run command
│   ├── check.go            # deploy check command
│   └── config.go           # deploy config command
├── deployers/
│   ├── registry.go         # Global deployer registry
│   ├── shell.go            # Built-in shell deployer
│   └── plugin.go           # External plugin executor (JSON protocol)
├── eval/
│   └── eval.go             # expr-lang expression evaluation
├── redact/
│   └── redact.go           # Secret value masking
├── resolver/
│   └── resolver.go         # Git-based plugin fetching and caching
└── plugins/                # Example plugins
    ├── ftp-deploy/         # FTP/FTPS upload plugin
    └── powershell-deploy/  # PowerShell script plugin
```

## License

MIT
