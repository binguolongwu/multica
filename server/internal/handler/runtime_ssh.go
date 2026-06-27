package handler

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"log/slog"
	"golang.org/x/crypto/ssh"
)

// ── SSH Connect Runtime ─────────────────────────────────────────────────────

// sshConnectRequest is the JSON body for POST /workspaces/{id}/runtimes/ssh-connect
type sshConnectRequest struct {
	Host      string   `json:"host"`
	Port      string   `json:"port"`
	Username  string   `json:"username"`
	Password  string   `json:"password"`
	Runtimes  []string `json:"runtimes"`
	ServerUrl string   `json:"server_url,omitempty"`
}

func (h *Handler) SSHConnectRuntime(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, wsID, "forbidden", "owner", "admin")
	if !ok {
		return
	}

	var req sshConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	req.Host = strings.TrimSpace(req.Host)
	req.Username = strings.TrimSpace(req.Username)
	req.Port = strings.TrimSpace(req.Port)
	if req.Host == "" {
		writeError(w, http.StatusBadRequest, "host is required")
		return
	}
	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	if len(req.Runtimes) == 0 {
		writeError(w, http.StatusBadRequest, "at least one runtime must be selected")
		return
	}
	if req.Port == "" {
		req.Port = "22"
	}

	// Locate the multica CLI binary on the server
	cliBinaryPath, err := findMulticaCLIBinary()
	if err != nil {
		slog.Warn("runtime_ssh: could not locate multica CLI binary", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"ok":    "false",
			"error": fmt.Sprintf("无法找到 multica CLI 程序: %v", err),
		})
		return
	}

	binaryData, err := os.ReadFile(cliBinaryPath)
	if err != nil {
		slog.Warn("runtime_ssh: failed to read multica CLI binary", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"ok":    "false",
			"error": fmt.Sprintf("无法读取 multica CLI 程序: %v", err),
		})
		return
	}

	// SSH connect with timeout
	sshConfig := &ssh.ClientConfig{
		User:            req.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(req.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}

	addr := net.JoinHostPort(req.Host, req.Port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		slog.Warn("runtime_ssh: failed to connect", "host", req.Host, "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"ok": "false", "error": fmt.Sprintf("SSH 连接失败: %v", err)})
		return
	}
	defer client.Close()

	// Step 1: Upload the multica CLI binary via stdin pipe
	slog.Info("runtime_ssh: uploading multica CLI binary", "host", req.Host, "size", len(binaryData))
	if err := uploadBinaryOverSSH(client, binaryData); err != nil {
		slog.Warn("runtime_ssh: failed to upload binary", "host", req.Host, "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"ok":    "false",
			"error": fmt.Sprintf("上传 multica CLI 失败: %v", err),
		})
		return
	}

	// Step 2: Run the bootstrap script to install and start
	serverUrl := resolveServerURL(r, req.ServerUrl)
	bootstrapScript := buildBootstrapScript(req.Runtimes, serverUrl)
	slog.Info("runtime_ssh: running bootstrap", "host", req.Host, "runtimes", req.Runtimes, "server_url", serverUrl)
	output, err := runRemoteCommand(client, bootstrapScript)
	if err != nil {
		slog.Warn("runtime_ssh: bootstrap failed", "host", req.Host, "error", err, "output", output)
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"ok":    "false",
			"error": fmt.Sprintf("远程执行失败: %v", err),
		})
		return
	}

	slog.Info("runtime_ssh: bootstrap completed", "host", req.Host)
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true", "host": req.Host})
}

// findMulticaCLIBinary locates the multica CLI binary on the server.
// It checks MULTICA_CLI_BINARY_PATH env var first, then looks in common locations.
func findMulticaCLIBinary() (string, error) {
	// Check env var
	if envPath := os.Getenv("MULTICA_CLI_BINARY_PATH"); envPath != "" {
		if _, statErr := os.Stat(envPath); statErr == nil {
			return envPath, nil
		} else {
			return "", fmt.Errorf("MULTICA_CLI_BINARY_PATH=%s not found: %w", envPath, statErr)
		}
	}

	// Check common paths
	candidates := []string{
		"bin/multica",                        // relative to CWD (dev mode)
		filepath.Join("server", "bin", "multica"), // relative to project root
		"/app/multica",                       // Docker
		"/usr/local/bin/multica",             // system install
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("multica CLI binary not found in any of: %v", candidates)
}

// uploadBinaryOverSSH pipes the multica CLI binary to the remote machine
// via the SSH session's stdin, writing it to /tmp/multica-cli.
func uploadBinaryOverSSH(client *ssh.Client, data []byte) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create SSH session: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("open stdin pipe: %w", err)
	}

	// Start the remote command that reads from stdin
	if err := session.Start("cat > /tmp/multica-cli && chmod +x /tmp/multica-cli"); err != nil {
		stdin.Close()
		return fmt.Errorf("start remote command: %w", err)
	}

	// Write the binary data
	if _, err := stdin.Write(data); err != nil {
		stdin.Close()
		return fmt.Errorf("write binary data: %w", err)
	}
	stdin.Close()

	// Wait for the command to complete
	if err := session.Wait(); err != nil {
		return fmt.Errorf("remote binary write failed: %w", err)
	}

	return nil
}

// resolveServerURL determines the server URL that the remote daemon should use to connect.
// Priority: request body server_url > REMOTE_API_URL env var > FRONTEND_ORIGIN env var > request Host header.
func resolveServerURL(r *http.Request, explicitUrl string) string {
	if explicitUrl != "" {
		return strings.TrimRight(explicitUrl, "/")
	}
	if url := os.Getenv("REMOTE_API_URL"); url != "" {
		return strings.TrimRight(url, "/")
	}
	if url := os.Getenv("FRONTEND_ORIGIN"); url != "" {
		return strings.TrimRight(url, "/")
	}
	// Fallback to the request's Host header (works when server is directly reachable)
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// buildBootstrapScript returns the shell script to install and start the selected runtimes.
func buildBootstrapScript(runtimes []string, serverUrl string) string {
	runtimeList := strings.Join(runtimes, " ")

	script := `#!/bin/bash
set -e

echo "[multica-ssh] Starting bootstrap..."
echo "[multica-ssh] Selected runtimes: ` + runtimeList + `"

# Move uploaded binary to system path
if [ -f /tmp/multica-cli ]; then
    echo "[multica-ssh] Installing multica CLI binary (size: $(stat -c%s /tmp/multica-cli 2>/dev/null || stat -f%z /tmp/multica-cli 2>/dev/null || echo unknown) bytes)..."
    mkdir -p /usr/local/bin 2>/dev/null || mkdir -p ~/.local/bin
    if [ -w /usr/local/bin ]; then
        mv /tmp/multica-cli /usr/local/bin/multica
        chmod +x /usr/local/bin/multica
        export PATH="/usr/local/bin:$PATH"
    else
        mkdir -p ~/.local/bin
        mv /tmp/multica-cli ~/.local/bin/multica
        chmod +x ~/.local/bin/multica
        export PATH="$HOME/.local/bin:$PATH"
    fi
    echo "[multica-ssh] multica CLI installed to $(which multica)"
else
    echo "[multica-ssh] FATAL: no binary uploaded — SSH upload must have failed"
    exit 1
fi

# Ensure multica is on PATH
export PATH="$HOME/.local/bin:/usr/local/bin:$PATH"

# Check multica is available
if ! command -v multica &> /dev/null; then
    echo "[multica-ssh] ERROR: multica CLI not found after installation"
    exit 1
fi

echo "[multica-ssh] multica CLI version: $(multica version 2>/dev/null || echo 'unknown')"

# Configure multica
echo "[multica-ssh] Setting up multica..."
echo "[multica-ssh] Server URL: ` + serverUrl + `"
multica config set server_url "` + serverUrl + `" 2>/dev/null || true

# Configure the selected runtimes and start daemon
echo "[multica-ssh] Configuring runtimes..."
for rt in ` + runtimeList + `; do
    echo "[multica-ssh] Enabling runtime: $rt"
    multica config set "runtimes.$rt.enabled" true 2>/dev/null || true
done

# Start the daemon in background
echo "[multica-ssh] Starting multica daemon..."
nohup multica daemon start > /tmp/multica-daemon.log 2>&1 &

echo "[multica-ssh] Bootstrap complete. Daemon started with PID $!"
echo "[multica-ssh] OK"
`
	return script
}

// runRemoteCommand executes a command over an existing SSH session and returns combined output.
func runRemoteCommand(client *ssh.Client, command string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	type result struct {
		output []byte
		err    error
	}
	done := make(chan result, 1)
	go func() {
		output, err := session.CombinedOutput(command)
		done <- result{output, err}
	}()

	select {
	case res := <-done:
		output := strings.TrimSpace(string(res.output))
		if res.err != nil {
			return output, fmt.Errorf("SSH command error: %w", res.err)
		}
		return output, nil
	case <-time.After(5 * time.Minute):
		return "", fmt.Errorf("SSH command timed out after 5 minutes")
	}
}
