package httpapi

import (
	"encoding/json"
	"fmt"
	"github.com/ironcore-dev/fabric-explorer/internal/observation"
	"github.com/ironcore-dev/fabric-explorer/internal/source"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func Handler(store *source.Store, webDir string) http.Handler {
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
			if r.Method != http.MethodHead {
				_ = json.NewEncoder(w).Encode(snapshot)
			}
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
