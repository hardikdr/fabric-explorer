package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ironcore-dev/fabric-explorer/internal/observation"
	"github.com/ironcore-dev/fabric-explorer/internal/source"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func Handler(store *source.Store, webDir string, observations ...*observation.Store) http.Handler {
	var obsStore *observation.Store
	if len(observations) > 0 {
		obsStore = observations[0]
	}
	files := http.FileServer(http.Dir(webDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; object-src 'none'")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Read-only service", http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/api/v1/inventory":
			p := store.Get()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("ETag", fmt.Sprintf("%q", fmt.Sprint(p.Revision)))
			if !p.Ready {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
			if r.Method != http.MethodHead {
				_, _ = w.Write(p.Body)
			}
		case "/api/v1/observations":
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			snapshot := observation.Snapshot{SchemaVersion: "v1", Mode: "disabled", Switches: []observation.Switch{}, Links: []observation.Link{}}
			if obsStore != nil {
				snapshot = obsStore.Get()
			}
			if r.Method != http.MethodHead {
				_ = json.NewEncoder(w).Encode(snapshot)
			}
		case "/api/v1/containers/logs":
			handleContainerLogs(w, r, obsStore)
		case "/healthz":
			w.WriteHeader(http.StatusOK)
		case "/readyz":
			if !store.Get().Ready {
				http.Error(w, "No usable resource snapshot", http.StatusServiceUnavailable)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		default:
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			path := filepath.Join(webDir, filepath.Clean("/"+r.URL.Path))
			if _, err := os.Stat(path); err != nil {
				if filepath.Ext(r.URL.Path) != "" {
					http.NotFound(w, r)
					return
				}
				http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
				return
			}
			files.ServeHTTP(w, r)
		}
	})
}

func handleContainerLogs(w http.ResponseWriter, r *http.Request, obsStore *observation.Store) {
	if obsStore == nil {
		http.Error(w, "observations not available", http.StatusServiceUnavailable)
		return
	}
	switchUID := r.URL.Query().Get("switchUID")
	containerID := r.URL.Query().Get("containerID")
	if switchUID == "" || containerID == "" {
		http.Error(w, "switchUID and containerID are required", http.StatusBadRequest)
		return
	}
	// Validate that the containerID was discovered for this switchUID in the current snapshot.
	// Use the canonical ID from the snapshot — not the raw HTTP value — to avoid injection.
	target, canonicalID, found := obsStore.ValidateContainerRequest(switchUID, containerID)
	if !found {
		http.Error(w, "container not found for this switch", http.StatusNotFound)
		return
	}
	reader, ok := obsStore.Reader.(observation.SSHReader)
	if !ok || reader.PasswordFile == "" {
		http.Error(w, "live SSH not configured", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	logs, errCode := reader.DockerLogs(ctx, target, canonicalID)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if errCode != "" && len(logs) == 0 {
		w.Header().Set("X-Log-Error", errCode)
		http.Error(w, "logs unavailable: "+errCode, http.StatusServiceUnavailable)
		return
	}
	if errCode != "" {
		w.Header().Set("X-Log-Error", errCode)
	}
	if r.Method != http.MethodHead {
		_, _ = w.Write(logs)
	}
}

