package correlate

import (
	"encoding/json"
	"fmt"
	"github.com/ironcore-dev/fabric-explorer/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"path"
	"regexp"
	"sort"
	"strings"
)

type Config struct {
	FabricletContainer string
	Mapping            map[string]string
	CellNamespaces     []string
}

func decode[T any](objects []unstructured.Unstructured) []T {
	out := make([]T, 0, len(objects))
	for _, o := range objects {
		b, _ := json.Marshal(o.Object)
		var v T
		_ = json.Unmarshal(b, &v)
		out = append(out, v)
	}
	return out
}
func diagnostic(code, summary string, related ...string) model.Diagnostic {
	return model.Diagnostic{Code: code, Severity: "warning", Summary: summary, Related: related}
}
func native(handle string) string {
	if strings.HasPrefix(handle, "sonic://") {
		return strings.TrimPrefix(handle, "sonic://")
	}
	return ""
}
func missing(kind, name string, states []model.SourceState) string {
	for _, s := range states {
		if s.Kind == kind && (!s.Complete || s.State != "ready") {
			return "Unavailable or incomplete " + kind + " source; reference " + name + " cannot currently be verified"
		}
	}
	return "Referenced " + kind + " " + name + " is missing"
}
func Correlate(raw map[string][]unstructured.Unstructured, states []model.SourceState, cfg Config) model.Snapshot {
	s := model.Snapshot{SchemaVersion: "v1", Sources: states, Switches: []model.SwitchView{}, Nodes: []model.NodeView{}, Cells: []model.CellView{}, Interfaces: decode[model.Interface](raw["interfaces"]), Diagnostics: []model.Diagnostic{}, Unmatched: map[string][]string{"nodes": {}, "cells": {}, "interfaces": {}}}
	nodes := decode[model.Node](raw["nodes"])
	cells := decode[model.Cell](raw["cells"])
	switches := decode[model.Switch](raw["switches"])
	ni := map[string]int{}
	ci := map[string]int{}
	ii := map[string]int{}
	handles := map[string][]int{}
	for i, n := range nodes {
		ni[n.Metadata.Name] = i
		s.Nodes = append(s.Nodes, model.NodeView{Resource: n, Binding: "unbound", CellKeys: []string{}, InterfaceKeys: []string{}, Diagnostics: []model.Diagnostic{}})
	}
	for i, in := range s.Interfaces {
		ii[in.Metadata.Name] = i
		handles[in.Spec.NodeRef.Name+"/"+in.Spec.Handle] = append(handles[in.Spec.NodeRef.Name+"/"+in.Spec.Handle], i)
		if n, ok := ni[in.Spec.NodeRef.Name]; ok {
			s.Nodes[n].InterfaceKeys = append(s.Nodes[n].InterfaceKeys, in.Metadata.Key())
		} else {
			s.Unmatched["interfaces"] = append(s.Unmatched["interfaces"], in.Metadata.Key())
			s.Diagnostics = append(s.Diagnostics, diagnostic("interface-node-missing", missing("nodes", in.Spec.NodeRef.Name, states), in.Metadata.Key()))
		}
	}
	for _, indices := range handles {
		if len(indices) > 1 {
			related := []string{}
			for _, i := range indices {
				related = append(related, s.Interfaces[i].Metadata.Key())
			}
			s.Diagnostics = append(s.Diagnostics, diagnostic("duplicate-interface-handle", "Multiple Interface objects claim the same Node and provider handle", related...))
		}
	}
	for i, c := range cells {
		ci[c.Metadata.Namespace+"/"+c.Metadata.Name] = i
		cv := model.CellView{Resource: c, Binding: "unbound", Role: c.Metadata.Labels["fabric.ironcore.dev/role"], Peers: []model.PeerView{}, Diagnostics: []model.Diagnostic{}}
		if n, ok := ni[c.Spec.NodeRef.Name]; ok {
			s.Nodes[n].CellKeys = append(s.Nodes[n].CellKeys, c.Metadata.Key())
		} else {
			cv.Diagnostics = append(cv.Diagnostics, diagnostic("cell-node-missing", missing("nodes", c.Spec.NodeRef.Name, states), c.Metadata.Key()))
			s.Unmatched["cells"] = append(s.Unmatched["cells"], c.Metadata.Key())
		}
		south := 0
		for _, p := range c.Spec.Peers {
			pv := model.PeerView{Peer: p, Purpose: "fabric", Resolution: "missing"}
			if p.DHCPRelay != "" {
				pv.Purpose = "south"
				south++
				if south < len(c.Spec.Prefixes) {
					pv.Prefix = c.Spec.Prefixes[south]
				} else {
					cv.Diagnostics = append(cv.Diagnostics, diagnostic("south-prefix-missing", "No ordered prefix for south-facing peer "+p.InterfaceRef.Name, c.Metadata.Key()))
				}
			}
			if in, ok := ii[p.InterfaceRef.Name]; ok {
				it := s.Interfaces[in]
				if it.Spec.NodeRef.Name == c.Spec.NodeRef.Name {
					pv.InterfaceKey = it.Metadata.Key()
					pv.Resolution = "resolved"
				} else {
					pv.Resolution = "wrong-node"
					cv.Diagnostics = append(cv.Diagnostics, diagnostic("interface-wrong-node", fmt.Sprintf("Interface %s belongs to Node %s, but Cell targets %s. Native-name registration may collide across switches.", p.InterfaceRef.Name, it.Spec.NodeRef.Name, c.Spec.NodeRef.Name), c.Metadata.Key(), it.Metadata.Key()))
				}
			} else {
				cv.Diagnostics = append(cv.Diagnostics, diagnostic("interface-missing", missing("interfaces", p.InterfaceRef.Name, states), c.Metadata.Key()))
			}
			cv.Peers = append(cv.Peers, pv)
		}
		s.Cells = append(s.Cells, cv)
	}
	for i, n := range nodes {
		v := &s.Nodes[i]
		if len(v.CellKeys) > 1 {
			v.Diagnostics = append(v.Diagnostics, diagnostic("multiple-targeted-cells", "Multiple Cells target this Node; only a UID-validated cellRef is bound", n.Metadata.Key()))
		}
		r := n.Spec.CellRef
		if r == nil {
			continue
		}
		v.Binding = "missing"
		if j, ok := ci[r.Namespace+"/"+r.Name]; ok {
			c := cells[j]
			switch {
			case r.UID == "" || r.UID != c.Metadata.UID:
				v.Binding = "uidMismatch"
				v.Diagnostics = append(v.Diagnostics, diagnostic("bound-cell-uid-mismatch", "Node cellRef UID does not match the current Cell; a recreated Cell is not the bound object", n.Metadata.Key(), c.Metadata.Key()))
			case c.Spec.NodeRef.Name != n.Metadata.Name:
				v.Binding = "reverseMismatch"
				v.Diagnostics = append(v.Diagnostics, diagnostic("bound-cell-node-mismatch", "Cell reverse nodeRef does not match this Node", n.Metadata.Key(), c.Metadata.Key()))
			default:
				v.Binding = "bound"
				v.BoundCellKey = c.Metadata.Key()
				s.Cells[j].Binding = "bound"
			}
		} else {
			summary := missing("cells", r.Namespace+"/"+r.Name, states)
			if len(cfg.CellNamespaces) > 0 {
				allowed := false
				for _, ns := range cfg.CellNamespaces {
					if ns == r.Namespace {
						allowed = true
					}
				}
				if !allowed {
					v.Binding = "inaccessible"
					summary = "Bound Cell namespace is outside the configured read scope"
				}
			}
			v.Diagnostics = append(v.Diagnostics, diagnostic("bound-cell-unresolved", summary, n.Metadata.Key()))
		}
	}
	claims := map[string][]int{}
	for i, sw := range switches {
		ev := []model.Evidence{}
		if mapped := cfg.Mapping[sw.Metadata.Name]; mapped != "" {
			ev = append(ev, model.Evidence{Type: "operator-configured", NodeName: mapped, Detail: "Explicit application mapping"})
		}
		// Inspect only the configured container and only its identity argument.
		if cfg.FabricletContainer != "" {
			cs, _, _ := unstructured.NestedSlice(raw["switches"][i].Object, "spec", "containers")
			for _, c := range cs {
				m, ok := c.(map[string]interface{})
				if !ok || m["name"] != cfg.FabricletContainer {
					continue
				}
				a, _, _ := unstructured.NestedStringSlice(m, "args")
				command, _, _ := unstructured.NestedStringSlice(m, "command")
				// Shell-wrapped bootstrap declarations occur in the lab. Recognize only
				// an explicit fabriclet executable and a literal DNS-style identity.
				// Never evaluate scripts, expansions, substitutions or arbitrary flags.
				if len(command) == 1 && (path.Base(command[0]) == "sh" || path.Base(command[0]) == "bash") && len(a) == 2 && a[0] == "-c" {
					for _, name := range shellBootstrapNames(a[1]) {
						ev = append(ev, model.Evidence{Type: "declared-shell-bootstrap", NodeName: name, Detail: "Recognized shell-wrapped Fabriclet declares a literal --name; execution is not confirmed"})
					}
				}
				a = append(command, a...)
				for j, arg := range a {
					name := ""
					if strings.HasPrefix(arg, "--name=") {
						name = strings.TrimPrefix(arg, "--name=")
					} else if arg == "--name" && j+1 < len(a) {
						name = a[j+1]
					}
					if name != "" && !strings.HasPrefix(name, "-") {
						ev = append(ev, model.Evidence{Type: "declared-bootstrap", NodeName: name, Detail: "Recognized container declares Fabriclet --name; execution is not confirmed"})
					}
				}
			}
		}
		if n, ok := ni[sw.Metadata.Name]; ok && nodes[n].Spec.ProviderID == "sonic://"+sw.Metadata.Name {
			ev = append(ev, model.Evidence{Type: "naming-convention", NodeName: sw.Metadata.Name, Detail: "Exact resource name and matching sonic:// providerID; not a hardware identity guarantee"})
		}
		a := model.Association{State: "unassociated", Candidates: []string{}, Evidence: ev}
		seen := map[string]bool{}
		for _, e := range ev {
			if !seen[e.NodeName] {
				a.Candidates = append(a.Candidates, e.NodeName)
				seen[e.NodeName] = true
			}
		}
		sort.Strings(a.Candidates)
		v := model.SwitchView{Resource: sw, DisplayName: sw.Spec.Hostname, Association: a, Role: "unknown", DeclaredPorts: []model.DeclaredPort{}, Diagnostics: []model.Diagnostic{}}
		if v.DisplayName == "" {
			v.DisplayName = sw.Metadata.Name
		}
		if len(a.Candidates) > 1 {
			v.Association.State = "ambiguous"
			v.Diagnostics = append(v.Diagnostics, diagnostic("switch-node-ambiguous", "Association evidence names different Nodes", sw.Metadata.Key()))
		} else if len(a.Candidates) == 1 {
			name := a.Candidates[0]
			if n, ok := ni[name]; ok {
				valid := nodes[n].Spec.ProviderID == "sonic://"+name
				configured := false
				for _, e := range ev {
					if e.Type == "operator-configured" {
						configured = true
					}
				}
				if valid || configured {
					v.Association.State = "associated"
					v.Association.NodeKey = nodes[n].Metadata.Key()
				} else {
					v.Association.State = "conflicting"
					v.Diagnostics = append(v.Diagnostics, diagnostic("provider-id-mismatch", "Declared Node does not have the expected sonic:// providerID", sw.Metadata.Key()))
				}
			} else {
				v.Association.State = "unresolved"
				v.Diagnostics = append(v.Diagnostics, diagnostic("switch-node-missing", missing("nodes", name, states), sw.Metadata.Key()))
			}
		}
		for _, candidate := range a.Candidates {
			claims[candidate] = append(claims[candidate], i)
		}
		s.Switches = append(s.Switches, v)
	}
	for _, indices := range claims {
		if len(indices) > 1 {
			for _, i := range indices {
				s.Switches[i].Association.State = "ambiguous"
				s.Switches[i].Association.NodeKey = ""
				s.Switches[i].Diagnostics = append(s.Switches[i].Diagnostics, diagnostic("node-multiple-switches", "Multiple Switches claim this Node; association remains unresolved", s.Switches[i].Resource.Metadata.Key()))
			}
		}
	}
	associated := map[string]bool{}
	for i := range s.Switches {
		v := &s.Switches[i]
		if v.Association.NodeKey != "" {
			associated[v.Association.NodeKey] = true
			for _, n := range s.Nodes {
				if n.Resource.Metadata.Key() != v.Association.NodeKey {
					continue
				}
				v.Diagnostics = append(v.Diagnostics, n.Diagnostics...)
				if n.BoundCellKey != "" {
					for _, c := range s.Cells {
						if c.Resource.Metadata.Key() == n.BoundCellKey {
							v.RoleCellKey = n.BoundCellKey
							if c.Role == "leaf" || c.Role == "spine" {
								v.Role = c.Role
							}
						}
					}
				}
			}
		}
		for _, p := range v.Resource.Spec.Ports {
			dp := model.DeclaredPort{Name: p.Name, Resolution: "unassociated"}
			if v.Association.NodeKey != "" {
				nodeName := v.Association.Candidates[0]
				matches := []model.Interface{}
				for _, in := range s.Interfaces {
					if in.Spec.NodeRef.Name == nodeName && native(in.Spec.Handle) == p.Name {
						matches = append(matches, in)
					}
				}
				switch len(matches) {
				case 0:
					dp.Resolution = "unregistered"
				case 1:
					dp.Resolution = "resolved"
					dp.InterfaceKey = matches[0].Metadata.Key()
				default:
					dp.Resolution = "conflicting"
					v.Diagnostics = append(v.Diagnostics, diagnostic("declared-port-conflict", "More than one local Interface matches declared port "+p.Name, v.Resource.Metadata.Key()))
				}
			}
			v.DeclaredPorts = append(v.DeclaredPorts, dp)
		}
	}
	for _, n := range s.Nodes {
		if !associated[n.Resource.Metadata.Key()] {
			s.Unmatched["nodes"] = append(s.Unmatched["nodes"], n.Resource.Metadata.Key())
			for _, key := range n.CellKeys {
				s.Unmatched["cells"] = append(s.Unmatched["cells"], key)
			}
			for _, key := range n.InterfaceKeys {
				s.Unmatched["interfaces"] = append(s.Unmatched["interfaces"], key)
			}
		}
	}
	for _, v := range s.Switches {
		s.Diagnostics = append(s.Diagnostics, v.Diagnostics...)
	}
	for _, v := range s.Cells {
		s.Diagnostics = append(s.Diagnostics, v.Diagnostics...)
	}
	for _, n := range s.Nodes {
		s.Diagnostics = append(s.Diagnostics, n.Diagnostics...)
	}
	unique := map[string]bool{}
	ds := make([]model.Diagnostic, 0, len(s.Diagnostics))
	for _, d := range s.Diagnostics {
		k := d.Code + d.Summary + strings.Join(d.Related, "/")
		if !unique[k] {
			unique[k] = true
			ds = append(ds, d)
		}
	}
	sort.Slice(ds, func(i, j int) bool {
		a, b := ds[i], ds[j]
		return a.Code+a.Summary+strings.Join(a.Related, "/") < b.Code+b.Summary+strings.Join(b.Related, "/")
	})
	s.Diagnostics = ds
	sort.Slice(s.Switches, func(i, j int) bool {
		a, b := s.Switches[i], s.Switches[j]
		if a.DisplayName == b.DisplayName {
			return a.Resource.Metadata.Key() < b.Resource.Metadata.Key()
		}
		if naturalLess(a.DisplayName, b.DisplayName) {
			return true
		}
		if naturalLess(b.DisplayName, a.DisplayName) {
			return false
		}
		return a.Resource.Metadata.Key() < b.Resource.Metadata.Key()
	})
	return s
}
func naturalLess(a, b string) bool {
	for len(a) > 0 && len(b) > 0 {
		if a[0] >= '0' && a[0] <= '9' && b[0] >= '0' && b[0] <= '9' {
			ai, bi := 0, 0
			for ai < len(a) && a[ai] >= '0' && a[ai] <= '9' {
				ai++
			}
			for bi < len(b) && b[bi] >= '0' && b[bi] <= '9' {
				bi++
			}
			x, y := strings.TrimLeft(a[:ai], "0"), strings.TrimLeft(b[:bi], "0")
			if len(x) != len(y) {
				return len(x) < len(y)
			}
			if x != y {
				return x < y
			}
			a, b = a[ai:], b[bi:]
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

var shellFabriclet = regexp.MustCompile(`(?:^|[\s;])(?:exec\s+)?(?:[./A-Za-z0-9_-]*/)?fabriclet(?:-sonic)?\s+([^;\n&|]+)`)
var shellIdentity = regexp.MustCompile(`(?:^|\s)--name(?:=|\s+)([a-z0-9](?:[a-z0-9.-]*[a-z0-9])?)(?:\s|$)`)

func shellBootstrapNames(script string) []string {
	names := []string{}
	for _, invocation := range shellFabriclet.FindAllStringSubmatch(script, -1) {
		for _, identity := range shellIdentity.FindAllStringSubmatch(invocation[1], -1) {
			names = append(names, identity[1])
		}
	}
	return names
}
