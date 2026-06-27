package handler

import (
	"net/http"
	"os"
	"strings"

	"log/slog"
)

// ── Self-hosted CLI Update Endpoints ────────────────────────────────────────

// HandleCLIVersion serves version information for self-hosted daemon updates.
// GET /api/daemon/cli-version
// Returns a GitHubRelease-compatible JSON with the server's current CLI version.
func (h *Handler) HandleCLIVersion(w http.ResponseWriter, r *http.Request) {
	version := resolveCLIVersion()

	// Build a response compatible with the cli.GitHubRelease shape
	resp := map[string]interface{}{
		"tag_name": "v" + version,
		"html_url": "",
		"assets":   []interface{}{},
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleCLIBinary serves the multica CLI binary for daemon self-update.
// GET /api/daemon/cli-binary?version=vX.Y.Z&os=linux&arch=amd64
func (h *Handler) HandleCLIBinary(w http.ResponseWriter, r *http.Request) {
	binaryPath, err := findMulticaCLIBinary()
	if err != nil {
		slog.Warn("cli_update: cli binary not found", "error", err)
		writeError(w, http.StatusNotFound, "CLI binary not available on this server")
		return
	}

	// Validate that the requested version matches the server's version.
	// For self-hosted servers, serve the current binary regardless of the
	// requested version — there is only one version available.
	requestedVersion := strings.TrimPrefix(strings.TrimSpace(r.URL.Query().Get("version")), "v")
	_ = requestedVersion

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=multica")
	http.ServeFile(w, r, binaryPath)
}

// resolveCLIVersion returns the CLI version based on the binary's modification time.
func resolveCLIVersion() string {
	binPath, err := findMulticaCLIBinary()
	if err != nil {
		return "unknown"
	}

	info, err := os.Stat(binPath)
	if err != nil {
		return "unknown"
	}

	return info.ModTime().Format("2006.01.021504")
}
