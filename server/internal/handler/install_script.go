package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ServeInstallScript serves the install.sh shell script from the repository's
// scripts/ directory. This lets self-hosted deployments provide a
// domain-relative URL (e.g. https://my-instance.com/install.sh) instead of
// pointing users to the upstream GitHub raw URL, which would be wrong for
// forks or private deployments.
//
// The script is read from disk on each request so operators can customise it
// without rebuilding the binary. Falls back to a minimal inline script if the
// file is missing.
func (h *Handler) ServeInstallScript(w http.ResponseWriter, r *http.Request) {
	// Try several candidate paths — covers `go run`, binary in server/bin,
	// and Docker working-directory layouts.
	candidates := []string{
		"scripts/install.sh",
		"../scripts/install.sh",
		"../../scripts/install.sh",
		filepath.Join(os.Getenv("MULTICA_INSTALL_SCRIPT_PATH"), "install.sh"),
	}

	for _, p := range candidates {
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}

	// Fallback: inline minimal installer so the endpoint never 404s.
	const fallback = `#!/usr/bin/env bash
set -euo pipefail
echo "Multica installer: scripts/install.sh not found on this server."
echo "Please install the CLI manually or contact your administrator."
exit 1
`
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fallback))
}

// resolvePublicBaseURL returns the best-guess public base URL of this
// server instance, derived from the request's Host header and the
// X-Forwarded-Proto header (or http if absent).
func resolvePublicBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	host := r.Host
	if host == "" {
		host = "localhost:8080"
	}
	return scheme + "://" + host
}

// InstallScriptURL returns the URL to this server's /install.sh endpoint,
// suitable for embedding in curl commands shown to the user.
func InstallScriptURL(r *http.Request) string {
	base := resolvePublicBaseURL(r)
	return strings.TrimRight(base, "/") + "/install.sh"
}
