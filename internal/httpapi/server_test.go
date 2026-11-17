package httpapi

import (
	"context"
	"encoding/json"
	"github.com/ironcore-dev/fabric-explorer/internal/correlate"
	"github.com/ironcore-dev/fabric-explorer/internal/model"
	"github.com/ironcore-dev/fabric-explorer/internal/observation"
	"github.com/ironcore-dev/fabric-explorer/internal/source"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestInventoryAndReadOnly(t *testing.T) {
	f, e := source.LoadFixture("../../testdata/kubernetes/clos.json")
	if e != nil {
		t.Fatal(e)
	}
	s := source.NewStore(f, correlate.Config{FabricletContainer: "fabriclet"}, "fixture", "fixture", time.Second, time.Second)
	s.Refresh(context.Background())
	h := Handler(s, "../../web/dist")
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE", "CONNECT"} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(method, "/api/v1/inventory", nil))
		if rr.Code != 405 {
			t.Fatal(method, rr.Code)
		}
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/inventory", nil))
	if rr.Code != 200 {
		t.Fatal(rr.Code)
	}
	var inventory model.Snapshot
	if e = json.Unmarshal(rr.Body.Bytes(), &inventory); e != nil || len(inventory.Switches) != 4 {
		t.Fatal(e)
	}
	for _, secret := range []string{"kubeconfig", "containers", "credentials", "scriptRef"} {
		if strings.Contains(rr.Body.String(), secret) {
			t.Fatal("sensitive fields", secret)
		}
	}
	if rr.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing CSP")
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/secrets", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatal(rr.Code)
	}
}
func TestUnavailableBody(t *testing.T) {
	f, e := source.LoadFixture("../../testdata/kubernetes/unavailable.json")
	if e != nil {
		t.Fatal(e)
	}
	s := source.NewStore(f, correlate.Config{}, "fixture", "fixture", time.Second, time.Second)
	s.Refresh(context.Background())
	h := Handler(s, "")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/inventory", nil))
	if rr.Code != 503 {
		t.Fatal(rr.Code)
	}
	var data model.Snapshot
	if json.Unmarshal(rr.Body.Bytes(), &data) != nil || data.Sources[0].State != "forbidden" || data.Sources[1].State != "missing-crd" {
		t.Fatal(rr.Body.String())
	}
}

func TestObservationEndpointReadOnly(t *testing.T) {
	f, e := source.LoadFixture("../../testdata/kubernetes/clos.json")
	if e != nil {
		t.Fatal(e)
	}
	s := source.NewStore(f, correlate.Config{}, "fixture", "test", time.Second, time.Second)
	s.Refresh(context.Background())
	h := Handler(s, "")
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE", "CONNECT"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "/api/v1/observations", strings.NewReader("reboot")))
		if w.Code != 405 {
			t.Fatal("write method accepted", method)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/observations?command=reboot", nil))
	var got map[string]any
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &got) != nil || got["mode"] != "disabled" || got["schemaVersion"] != "v1" {
		t.Fatal("disabled contract invalid")
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("sample caching allowed")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("HEAD", "/api/v1/observations", nil))
	if w.Body.Len() != 0 {
		t.Fatal("HEAD body exposed")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/commands", nil))
	if w.Code != 404 {
		t.Fatal("unexpected command endpoint")
	}
}

func TestContainerLogsEndpoint(t *testing.T) {
	f, e := source.LoadFixture("../../testdata/kubernetes/clos.json")
	if e != nil {
		t.Fatal(e)
	}
	s := source.NewStore(f, correlate.Config{}, "fixture", "test", time.Second, time.Second)
	s.Refresh(context.Background())
	h := Handler(s, "")

	// No observation store → 503.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/containers/logs?switchUID=x&containerID=y", nil))
	if w.Code != 503 {
		t.Fatalf("expected 503 without obs store, got %d", w.Code)
	}

	// With a nil observation store: same 503.
	h2 := Handler(s, "", nil)
	w = httptest.NewRecorder()
	h2.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/containers/logs?switchUID=x&containerID=y", nil))
	if w.Code != 503 {
		t.Fatalf("expected 503 with nil obs store, got %d", w.Code)
	}

	// With an obs store (reader=nil → no SSH): validating missing params.
	obsStore := observation.NewStore(nil, s, "fixture", time.Second, time.Second, 10*time.Second)
	h3 := Handler(s, "", obsStore)

	// Missing switchUID → 400.
	w = httptest.NewRecorder()
	h3.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/containers/logs?containerID=abc", nil))
	if w.Code != 400 {
		t.Fatalf("expected 400 for missing switchUID, got %d", w.Code)
	}

	// Missing containerID → 400.
	w = httptest.NewRecorder()
	h3.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/containers/logs?switchUID=abc", nil))
	if w.Code != 400 {
		t.Fatalf("expected 400 for missing containerID, got %d", w.Code)
	}

	// Unknown switchUID → 404 (not in observation snapshot).
	w = httptest.NewRecorder()
	h3.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/containers/logs?switchUID=nosuchuid&containerID=abc", nil))
	if w.Code != 404 {
		t.Fatalf("expected 404 for unknown switchUID, got %d", w.Code)
	}

	// POST → 405 (write rejected).
	w = httptest.NewRecorder()
	h3.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/containers/logs?switchUID=x&containerID=y", strings.NewReader("exec")))
	if w.Code != 405 {
		t.Fatalf("expected 405 for POST, got %d", w.Code)
	}
}
