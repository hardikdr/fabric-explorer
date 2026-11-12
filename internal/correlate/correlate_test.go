package correlate_test

import (
	"encoding/json"
	"github.com/ironcore-dev/fabric-explorer/internal/correlate"
	"github.com/ironcore-dev/fabric-explorer/internal/model"
	"github.com/ironcore-dev/fabric-explorer/internal/source"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) source.Fixture {
	t.Helper()
	f, e := source.LoadFixture("../../testdata/kubernetes/" + name + ".json")
	if e != nil {
		t.Fatal(e)
	}
	return f
}
func build(f source.Fixture) model.Snapshot {
	return correlate.Correlate(f.Resources, nil, correlate.Config{FabricletContainer: "fabriclet"})
}
func TestClos(t *testing.T) {
	s := build(fixture(t, "clos"))
	if len(s.Switches) != 4 || len(s.Cells) != 4 || len(s.Interfaces) != 12 {
		t.Fatal("fixture counts", len(s.Interfaces))
	}
	for _, sw := range s.Switches {
		if sw.Association.State != "associated" || sw.Role == "unknown" {
			t.Fatal(sw)
		}
	}
	for _, c := range s.Cells {
		if c.Binding != "bound" {
			t.Fatal(c)
		}
		south := 0
		for _, p := range c.Peers {
			if p.Resolution != "resolved" {
				t.Fatal(p)
			}
			if p.Purpose == "south" {
				south++
				if p.Prefix != c.Resource.Spec.Prefixes[south] {
					t.Fatal("prefix order")
				}
			}
		}
	}
}
func TestBindingIdentity(t *testing.T) {
	for _, tt := range []struct {
		name, state string
		mutate      func(source.Fixture)
	}{
		{"uid", "uidMismatch", func(f source.Fixture) {
			_ = unstructured.SetNestedField(f.Resources["nodes"][0].Object, "old-uid", "spec", "cellRef", "uid")
		}},
		{"reverse", "reverseMismatch", func(f source.Fixture) {
			_ = unstructured.SetNestedField(f.Resources["cells"][0].Object, "leaf-1", "spec", "nodeRef", "name")
		}},
		{"namespace", "missing", func(f source.Fixture) {
			_ = unstructured.SetNestedField(f.Resources["nodes"][0].Object, "other-ns", "spec", "cellRef", "namespace")
		}},
		{"name", "missing", func(f source.Fixture) {
			_ = unstructured.SetNestedField(f.Resources["nodes"][0].Object, "other-name", "spec", "cellRef", "name")
		}},
		{"no-ref", "unbound", func(f source.Fixture) {
			unstructured.RemoveNestedField(f.Resources["nodes"][0].Object, "spec", "cellRef")
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := fixture(t, "clos")
			tt.mutate(f)
			s := build(f)
			if s.Nodes[0].Binding != tt.state || s.Nodes[0].BoundCellKey != "" {
				t.Fatal(s.Nodes[0])
			}
			for _, sw := range s.Switches {
				if sw.Resource.Metadata.Name == "spine-1" && sw.Role != "unknown" {
					t.Fatal("role inferred from unbound Cell")
				}
			}
		})
	}
}
func TestIncompleteAndScope(t *testing.T) {
	f := fixture(t, "clos")
	f.Resources["cells"] = nil
	states := []model.SourceState{{Kind: "cells", State: "forbidden", Complete: false}}
	s := correlate.Correlate(f.Resources, states, correlate.Config{CellNamespaces: []string{"different"}})
	if s.Nodes[0].Binding != "inaccessible" {
		t.Fatal(s.Nodes[0])
	}
	s = correlate.Correlate(f.Resources, states, correlate.Config{})
	if !strings.Contains(s.Nodes[0].Diagnostics[0].Summary, "incomplete") {
		t.Fatal(s.Nodes[0])
	}
}
func TestEdgeCases(t *testing.T) {
	s := build(fixture(t, "edge-cases"))
	byName := map[string]model.SwitchView{}
	for _, sw := range s.Switches {
		byName[sw.Resource.Metadata.Name] = sw
	}
	if byName["leaf-1"].Association.State != "ambiguous" || byName["duplicate-leaf"].Association.State != "ambiguous" {
		t.Fatal("multiple switch claim not detected")
	}
	if byName["unregistered"].Association.State != "unresolved" || byName["leaf-2"].Role != "unknown" {
		t.Fatal("missing status/role handling")
	}
	if len(s.Unmatched["nodes"]) == 0 || len(s.Unmatched["cells"]) == 0 || len(s.Unmatched["interfaces"]) == 0 {
		t.Fatal("orphans hidden")
	}
	codes := map[string]bool{}
	for _, d := range s.Diagnostics {
		codes[d.Code] = true
	}
	for _, code := range []string{"interface-wrong-node", "interface-missing", "duplicate-interface-handle", "bound-cell-uid-mismatch", "multiple-targeted-cells"} {
		if !codes[code] {
			t.Fatal("missing diagnostic", code)
		}
	}
	for _, c := range s.Cells {
		if c.Resource.Metadata.Name == "leaf-1-config" {
			if c.Peers[2].Resolution != "wrong-node" || c.Peers[2].Prefix != "10.1.1.0/24" || c.Peers[3].Prefix != "10.1.2.0/24" {
				t.Fatal("prefix shifted by unresolved interface")
			}
		}
	}
}
func TestEvidenceConflictsAndSanitization(t *testing.T) {
	f := fixture(t, "clos")
	_ = unstructured.SetNestedStringSlice(f.Resources["switches"][0].Object, []string{"--name=leaf-1", "--token=never-expose"}, "spec", "containers-placeholder")
	cs, _, _ := unstructured.NestedSlice(f.Resources["switches"][0].Object, "spec", "containers")
	cs[0].(map[string]interface{})["args"] = []interface{}{"--name=leaf-1", "--token=never-expose"}
	_ = unstructured.SetNestedSlice(f.Resources["switches"][0].Object, cs, "spec", "containers")
	s := build(f)
	for _, sw := range s.Switches {
		if sw.Resource.Metadata.Name == "spine-1" && sw.Association.State != "ambiguous" {
			t.Fatal(sw)
		}
	}
	b, _ := json.Marshal(s)
	for _, secret := range []string{"never-expose", "kubeconfig", "containers", "credentials"} {
		if strings.Contains(string(b), secret) {
			t.Fatal("sensitive content exposed", secret)
		}
	}
}
func TestRecognitionAndMapping(t *testing.T) {
	f := fixture(t, "clos")
	f.Resources["switches"][0].SetName("different-switch")
	s := correlate.Correlate(f.Resources, nil, correlate.Config{FabricletContainer: "unrecognized"})
	for _, sw := range s.Switches {
		if sw.Resource.Metadata.Name == "different-switch" && sw.Association.State != "unassociated" {
			t.Fatal(sw)
		}
	}
	s = correlate.Correlate(f.Resources, nil, correlate.Config{Mapping: map[string]string{"different-switch": "spine-1"}})
	for _, sw := range s.Switches {
		if sw.Resource.Metadata.Name == "different-switch" && sw.Association.State != "associated" {
			t.Fatal(sw)
		}
	}
}
func TestFixtureShape(t *testing.T) {
	b, e := os.ReadFile("../../testdata/kubernetes/clos.json")
	if e != nil {
		t.Fatal(e)
	}
	if !json.Valid(b) {
		t.Fatal("invalid JSON")
	}
}

func TestShellWrappedBootstrapAndRedaction(t *testing.T) {
	f := fixture(t, "clos")
	sw := &f.Resources["switches"][0]
	cs, _, _ := unstructured.NestedSlice(sw.Object, "spec", "containers")
	for _, c := range cs {
		m := c.(map[string]interface{})
		if m["name"] == "fabriclet" {
			m["command"] = []interface{}{"/bin/sh"}
			m["args"] = []interface{}{"-c", "nsenter --target 1 --net /fabriclet --name " + sw.GetName() + " --backend configdb; credential=never-publish-this"}
		}
	}
	_ = unstructured.SetNestedSlice(sw.Object, cs, "spec", "containers")
	snapshot := build(f)
	found := false
	for _, v := range snapshot.Switches {
		if v.Resource.Metadata.Name == sw.GetName() {
			if v.Association.State != "associated" {
				t.Fatal(v.Association)
			}
			for _, e := range v.Association.Evidence {
				found = found || e.Type == "declared-shell-bootstrap"
			}
		}
	}
	b, _ := json.Marshal(snapshot)
	if !found || strings.Contains(string(b), "never-publish-this") || strings.Contains(string(b), "nsenter") {
		t.Fatal("missing evidence or script leaked")
	}
}
