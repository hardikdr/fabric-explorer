package httpapi

import (
	"context"
	"encoding/json"
	"github.com/ironcore-dev/fabric-explorer/internal/correlate"
	"github.com/ironcore-dev/fabric-explorer/internal/model"
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
