// ftp-deploy is an external deployer plugin for uploading files to FTP servers.
//
// It supports both plain FTP (via the Go FTP library) and FTPS (via curl,
// which handles TLS session reuse natively). Files and directories can be
// uploaded recursively.
//
// Config keys:
//
//	host        - FTP server hostname (required)
//	port        - FTP server port (default: "21")
//	username    - FTP username (default: "anonymous")
//	password    - FTP password
//	local_path  - Local file or directory to upload (required)
//	remote_path - Remote destination path (default: "/")
//	tls         - Set to "true" to enable FTPS via curl
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
)

// PluginRequest is the JSON payload received from the deploy tool on stdin.
type PluginRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Config      map[string]string `json:"config"`
	Env         map[string]string `json:"env"`
}

// PluginResponse is the JSON payload sent back to the deploy tool on stdout.
type PluginResponse struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}

func main() {
	var req PluginRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		fatal("failed to read request: " + err.Error())
	}

	// Expand env vars in all config values
	for k, v := range req.Config {
		req.Config[k] = expandEnv(v, req.Env)
	}

	host := req.Config["host"]
	port := req.Config["port"]
	username := req.Config["username"]
	password := req.Config["password"]
	localPath := req.Config["local_path"]
	remotePath := req.Config["remote_path"]
	useTLS := strings.EqualFold(req.Config["tls"], "true")

	if host == "" {
		fatal("config 'host' is required")
	}
	if port == "" {
		port = "21"
	}
	if username == "" {
		username = "anonymous"
	}
	if remotePath == "" {
		remotePath = "/"
	}
	if localPath == "" {
		fatal("config 'local_path' is required")
	}

	// Use curl for FTPS (TLS) — it handles session reuse natively.
	// Fall back to Go FTP library for plain FTP.
	if useTLS {
		uploadWithCurl(host, port, username, password, localPath, remotePath)
	} else {
		uploadWithGoFTP(host, port, username, password, localPath, remotePath)
	}
}

// --- curl-based FTPS upload (handles TLS session reuse) ---

// uploadWithCurl uses the system curl command for FTPS uploads.
// curl handles TLS session reuse natively, avoiding the limitations
// of Go FTP libraries with servers that require session reuse.
func uploadWithCurl(host, port, username, password, localPath, remotePath string) {
	var out strings.Builder

	info, err := os.Stat(localPath)
	if err != nil {
		fatal(fmt.Sprintf("local path %q: %v", localPath, err))
	}

	var files []string
	if info.IsDir() {
		filepath.Walk(localPath, func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return err
			}
			files = append(files, path)
			return nil
		})
	} else {
		files = []string{localPath}
	}

	for _, file := range files {
		var dest string
		if info.IsDir() {
			rel, _ := filepath.Rel(localPath, file)
			dest = remotePath + "/" + filepath.ToSlash(rel)
		} else if strings.HasSuffix(remotePath, "/") {
			dest = remotePath + filepath.Base(file)
		} else {
			dest = remotePath
		}

		ftpURL := fmt.Sprintf("ftp://%s:%s%s", host, port, dest)

		out.WriteString(fmt.Sprintf("Uploading %s -> %s (FTPS via curl)\n", file, dest))

		args := []string{
			"--ssl-reqd", // require TLS
			"--ftp-ssl-ccc-mode", "passive",
			"-u", username + ":" + password,
			"-T", file,
			"--ftp-create-dirs",
			ftpURL,
		}

		cmd := exec.Command("curl", args...)
		cmdOut, err := cmd.CombinedOutput()
		out.Write(cmdOut)

		if err != nil {
			respond(PluginResponse{
				Success: false,
				Output:  out.String(),
				Error:   fmt.Sprintf("curl upload failed for %s: %v", file, err),
			})
			return
		}
	}

	out.WriteString(fmt.Sprintf("Uploaded %d file(s) successfully.\n", len(files)))
	respond(PluginResponse{
		Success: true,
		Output:  out.String(),
	})
}

// --- Go FTP library for plain (non-TLS) FTP ---

// uploadWithGoFTP uses the jlaffaye/ftp library for plain (non-TLS) FTP uploads.
func uploadWithGoFTP(host, port, username, password, localPath, remotePath string) {
	var out strings.Builder
	addr := fmt.Sprintf("%s:%s", host, port)

	out.WriteString(fmt.Sprintf("Connecting to %s...\n", addr))

	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(30*time.Second))
	if err != nil {
		fatal(fmt.Sprintf("failed to connect to %s: %v", addr, err))
	}
	defer conn.Quit()

	out.WriteString("Connected. Logging in...\n")

	if err := conn.Login(username, password); err != nil {
		fatal(fmt.Sprintf("login failed: %v", err))
	}

	out.WriteString(fmt.Sprintf("Logged in as %s\n", username))

	info, err := os.Stat(localPath)
	if err != nil {
		fatal(fmt.Sprintf("local path %q: %v", localPath, err))
	}

	if info.IsDir() {
		count, err := uploadDir(conn, localPath, remotePath, &out)
		if err != nil {
			respond(PluginResponse{
				Success: false,
				Output:  out.String(),
				Error:   err.Error(),
			})
			return
		}
		out.WriteString(fmt.Sprintf("Uploaded %d files to %s\n", count, remotePath))
	} else {
		if err := uploadFile(conn, localPath, remotePath, &out); err != nil {
			respond(PluginResponse{
				Success: false,
				Output:  out.String(),
				Error:   err.Error(),
			})
			return
		}
	}

	respond(PluginResponse{
		Success: true,
		Output:  out.String(),
	})
}

// uploadFile uploads a single file to the FTP server, creating parent directories as needed.
func uploadFile(conn *ftp.ServerConn, localPath, remotePath string, out *strings.Builder) error {
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", localPath, err)
	}
	defer f.Close()

	if strings.HasSuffix(remotePath, "/") {
		remotePath = remotePath + filepath.Base(localPath)
	}

	parentDir := filepath.ToSlash(filepath.Dir(remotePath))
	mkdirAll(conn, parentDir, out)

	out.WriteString(fmt.Sprintf("  Uploading %s -> %s\n", localPath, remotePath))
	if err := conn.Stor(remotePath, f); err != nil {
		return fmt.Errorf("upload %s: %w", remotePath, err)
	}
	return nil
}

// uploadDir recursively uploads all files in a local directory to the FTP server.
func uploadDir(conn *ftp.ServerConn, localDir, remoteDir string, out *strings.Builder) (int, error) {
	count := 0

	err := filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(localDir, path)
		remoteFull := remoteDir + "/" + filepath.ToSlash(relPath)

		if info.IsDir() {
			out.WriteString(fmt.Sprintf("  Creating directory %s\n", remoteFull))
			conn.MakeDir(remoteFull)
			return nil
		}

		if err := uploadFile(conn, path, remoteFull, out); err != nil {
			return err
		}
		count++
		return nil
	})

	return count, err
}

// mkdirAll creates all directories in a path on the FTP server.
func mkdirAll(conn *ftp.ServerConn, dir string, out *strings.Builder) {
	dir = filepath.ToSlash(dir)
	if dir == "/" || dir == "." || dir == "" {
		return
	}
	parts := strings.Split(strings.Trim(dir, "/"), "/")
	current := ""
	for _, p := range parts {
		current += "/" + p
		if err := conn.MakeDir(current); err == nil {
			out.WriteString(fmt.Sprintf("  Created directory %s\n", current))
		}
	}
}

// expandEnv replaces ${KEY} and ${env.KEY} placeholders with values from the env map.
func expandEnv(s string, env map[string]string) string {
	for k, v := range env {
		s = strings.ReplaceAll(s, "${env."+k+"}", v)
		s = strings.ReplaceAll(s, "${"+k+"}", v)
	}
	return s
}

// respond writes a JSON PluginResponse to stdout.
func respond(resp PluginResponse) {
	json.NewEncoder(os.Stdout).Encode(resp)
}

// fatal sends an error response and exits.
func fatal(msg string) {
	respond(PluginResponse{
		Success: false,
		Error:   msg,
	})
	os.Exit(1)
}
