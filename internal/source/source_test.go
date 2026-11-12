package source

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ironcore-dev/fabric-explorer/internal/correlate"
	"github.com/ironcore-dev/fabric-explorer/internal/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	kt "k8s.io/client-go/testing"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReaderErrorsAndScope(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		code string
	}{{"forbidden", apierrors.NewForbidden(GVRs["cells"].GroupResource(), "", errors.New("secret-detail")), "forbidden"}, {"missing", apierrors.NewNotFound(GVRs["cells"].GroupResource(), ""), "missing-crd"}, {"unavailable", errors.New("secret-url"), "unavailable"}} {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{GVRs["cells"]: "CellList"})
			client.PrependReactor("list", "cells", func(a kt.Action) (bool, runtime.Object, error) {
				if a.GetNamespace() != "lab" {
					t.Fatal("scope not enforced")
				}
				return true, nil, tt.err
			})
			r := KubeReader{Client: client, Namespaces: []string{"lab"}}
			out := r.Read(context.Background(), "cells")
			if out.Error != tt.code || out.Complete {
				t.Fatal(out)
			}
			for _, a := range client.Actions() {
				if a.GetVerb() != "list" {
					t.Fatal("write action", a)
				}
			}
		})
	}
	_ = time.Second
}
func TestPaginationAndPartial(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "partial"}[fail], func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Query().Get("limit") != "1" {
					t.Error("not a bounded read")
				}
				w.Header().Set("Content-Type", "application/json")
				if calls == 2 && fail {
					http.Error(w, "Unavailable", 503)
					return
				}
				cont := "next"
				if calls == 2 {
					cont = ""
					if r.URL.Query().Get("continue") != "next" {
						t.Error("missing continuation")
					}
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"apiVersion": "fabric.ironcore.dev/v1alpha1", "kind": "NodeList", "metadata": map[string]interface{}{"continue": cont, "resourceVersion": "100"}, "items": []interface{}{map[string]interface{}{"apiVersion": "fabric.ironcore.dev/v1alpha1", "kind": "Node", "metadata": map[string]interface{}{"name": map[bool]string{true: "n2", false: "n1"}[calls == 2]}}}})
			}))
			defer server.Close()
			client, err := dynamic.NewForConfig(&rest.Config{Host: server.URL, Timeout: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			r := KubeReader{Client: client, Limit: 1, MaxObjects: 10}
			out := r.Read(context.Background(), "nodes")
			if out.Complete == fail {
				t.Fatal(out)
			}
			want := 2
			if fail {
				want = 1
			}
			if len(out.Items) != want {
				t.Fatal(out)
			}
			if calls < 2 {
				t.Fatal("pagination not followed")
			}
		})
	}
}
func TestObjectBound(t *testing.T) {
	f, err := LoadFixture("../../testdata/kubernetes/clos.json")
	if err != nil {
		t.Fatal(err)
	}
	objects := []runtime.Object{}
	for _, o := range f.Resources["nodes"] {
		o := o
		objects = append(objects, &o)
	}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{GVRs["nodes"]: "NodeList"}, objects...)
	r := KubeReader{Client: client, MaxObjects: 2}
	out := r.Read(context.Background(), "nodes")
	if len(out.Items) != 2 || out.Complete || out.Error != "object-limit" {
		t.Fatal(out)
	}
}

type mutableReader struct {
	fixture Fixture
	fail    bool
	partial bool
}

func (r *mutableReader) Read(ctx context.Context, kind string) Result {
	if r.fail {
		return Result{Complete: false, Error: "forbidden"}
	}
	result := r.fixture.Read(ctx, kind)
	if r.partial {
		result.Items = result.Items[:min(1, len(result.Items))]
		result.Complete = false
		result.Error = "unavailable"
	}
	return result
}
func TestStoreRetentionAndRecovery(t *testing.T) {
	f, e := LoadFixture("../../testdata/kubernetes/clos.json")
	if e != nil {
		t.Fatal(e)
	}
	r := &mutableReader{fixture: f}
	s := NewStore(r, correlate.Config{}, "fixture", "test", time.Second, time.Second)
	if s.Get().Ready {
		t.Fatal("initial loading should not be ready")
	}
	s.Refresh(context.Background())
	old := s.Get()
	var before model.Snapshot
	_ = json.Unmarshal(old.Body, &before)
	r.fail = true
	s.Refresh(context.Background())
	var after model.Snapshot
	_ = json.Unmarshal(s.Get().Body, &after)
	if !s.Get().Ready || len(after.Switches) != 4 {
		t.Fatal("lost last-good snapshot")
	}
	for _, st := range after.Sources {
		if st.State != "stale" || st.Complete || st.Error != "forbidden" || st.LastSuccessfulListAt == "" {
			t.Fatal(st)
		}
	}
	var immutable model.Snapshot
	_ = json.Unmarshal(old.Body, &immutable)
	if immutable.Sources[0].State != "ready" {
		t.Fatal("published snapshot mutated")
	}
	r.fail = false
	r.partial = true
	s.Refresh(context.Background())
	_ = json.Unmarshal(s.Get().Body, &after)
	if len(after.Switches) != 4 || after.Sources[0].State != "partial" {
		t.Fatal("partial list removed previous objects")
	}
	r.partial = false
	r.fixture.Resources["switches"] = []unstructured.Unstructured{}
	s.Refresh(context.Background())
	_ = json.Unmarshal(s.Get().Body, &after)
	if len(after.Switches) != 0 || !after.Sources[0].Complete {
		t.Fatal("complete empty list must remove old objects")
	}
}
func TestInitialFailureDistinctFromEmpty(t *testing.T) {
	for _, name := range []string{"unavailable", "empty"} {
		f, e := LoadFixture("../../testdata/kubernetes/" + name + ".json")
		if e != nil {
			t.Fatal(e)
		}
		s := NewStore(f, correlate.Config{}, "fixture", "test", time.Second, time.Second)
		s.Refresh(context.Background())
		if s.Get().Ready != (name == "empty") {
			t.Fatal(name)
		}
	}
}
func TestIncompatibleSchemaRetainsKnownData(t *testing.T) {
	f, err := LoadFixture("../../testdata/kubernetes/clos.json")
	if err != nil {
		t.Fatal(err)
	}
	r := &mutableReader{fixture: f}
	s := NewStore(r, correlate.Config{}, "fixture", "test", time.Second, time.Second)
	s.Refresh(context.Background())
	_ = unstructured.SetNestedField(f.Resources["switches"][0].Object, int64(42), "spec", "management", "host")
	s.Refresh(context.Background())
	var result model.Snapshot
	_ = json.Unmarshal(s.Get().Body, &result)
	if len(result.Switches) != 4 || result.Sources[0].Error != "incompatible-schema" || result.Sources[0].Complete {
		t.Fatal("malformed object silently treated as current", result.Sources[0])
	}
}
func TestFixtureNamespaceScope(t *testing.T) {
	f := FileFixture{Path: "../../testdata/kubernetes/clos.json", Namespaces: []string{"other-namespace"}}
	cells := f.Read(context.Background(), "cells")
	if !cells.Complete || len(cells.Items) != 0 {
		t.Fatal("fixture scope leaked cells", cells)
	}
	if len(f.Read(context.Background(), "switches").Items) != 4 {
		t.Fatal("scope hid cluster-scoped switches")
	}
	s := NewStore(f, correlate.Config{CellNamespaces: f.Namespaces}, "fixture", "test", time.Second, time.Second)
	s.Refresh(context.Background())
	var snapshot model.Snapshot
	_ = json.Unmarshal(s.Get().Body, &snapshot)
	for _, n := range snapshot.Nodes {
		if n.Binding != "inaccessible" {
			t.Fatal(n)
		}
	}
}
