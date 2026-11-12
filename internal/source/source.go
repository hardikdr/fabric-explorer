package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ironcore-dev/fabric-explorer/internal/correlate"
	"github.com/ironcore-dev/fabric-explorer/internal/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var Kinds = []string{"switches", "nodes", "interfaces", "cells"}
var GVRs = map[string]schema.GroupVersionResource{
	"switches":   {Group: "sonic.networking.metal.ironcore.dev", Version: "v1alpha1", Resource: "switches"},
	"nodes":      {Group: "fabric.ironcore.dev", Version: "v1alpha1", Resource: "nodes"},
	"interfaces": {Group: "fabric.ironcore.dev", Version: "v1alpha1", Resource: "interfaces"},
	"cells":      {Group: "fabric.ironcore.dev", Version: "v1alpha1", Resource: "cells"},
}

type Result struct {
	Items    []unstructured.Unstructured
	Complete bool
	Error    string
	Version  string
}
type Reader interface {
	Read(context.Context, string) Result
}
type KubeReader struct {
	Client     dynamic.Interface
	Namespaces []string
	Limit      int64
	MaxObjects int
}

func (r *KubeReader) Read(ctx context.Context, kind string) Result {
	res := Result{Items: []unstructured.Unstructured{}, Complete: true}
	scopes := []string{""}
	if kind == "cells" && len(r.Namespaces) > 0 {
		scopes = r.Namespaces
	}
	for _, scope := range scopes {
		var client dynamic.ResourceInterface = r.Client.Resource(GVRs[kind])
		if kind == "cells" {
			client = r.Client.Resource(GVRs[kind]).Namespace(scope)
		}
		token := ""
		seen := map[string]bool{}
		for {
			limit := r.Limit
			if limit <= 0 {
				limit = 500
			}
			list, err := client.List(ctx, metav1.ListOptions{Limit: limit, Continue: token})
			if err != nil {
				res.Complete = false
				switch {
				case apierrors.IsForbidden(err):
					res.Error = "forbidden"
				case apierrors.IsNotFound(err):
					res.Error = "missing-crd"
				case apierrors.IsResourceExpired(err):
					res.Error = "expired-list"
				default:
					res.Error = "unavailable"
				}
				return res
			}
			max := r.MaxObjects
			if max <= 0 {
				max = 10000
			}
			remaining := max - len(res.Items)
			if len(list.Items) > remaining {
				res.Items = append(res.Items, list.Items[:remaining]...)
				res.Complete = false
				res.Error = "object-limit"
				return res
			}
			res.Items = append(res.Items, list.Items...)
			res.Version = list.GetResourceVersion()
			token = list.GetContinue()
			if token == "" {
				break
			}
			if seen[token] {
				res.Complete = false
				res.Error = "invalid-pagination"
				return res
			}
			seen[token] = true
		}
	}
	// Multiple namespace list versions are not a coherent version; do not imply one.
	if len(scopes) > 1 {
		res.Version = ""
	}
	return res
}

// Fixtures are CRD-shaped objects; sourceErrors simulate missing/partial sources.
// FileFixture rereads a local fixture on each poll, allowing safe stale/recovery demos.
type FileFixture struct {
	Path       string
	Namespaces []string
}

func (f FileFixture) Read(ctx context.Context, kind string) Result {
	fixture, err := LoadFixture(f.Path)
	if err != nil {
		return Result{Error: "unavailable", Complete: false}
	}
	result := fixture.Read(ctx, kind)
	if kind == "cells" && len(f.Namespaces) > 0 {
		allowed := map[string]bool{}
		for _, ns := range f.Namespaces {
			allowed[ns] = true
		}
		scoped := make([]unstructured.Unstructured, 0, len(result.Items))
		for _, o := range result.Items {
			if allowed[o.GetNamespace()] {
				scoped = append(scoped, o)
			}
		}
		result.Items = scoped
	}
	return result
}

type Fixture struct {
	Resources    map[string][]unstructured.Unstructured
	SourceErrors map[string]string
	PartialKinds []string
}

func (f Fixture) Read(ctx context.Context, kind string) Result {
	err := f.SourceErrors[kind]
	complete := err == ""
	for _, k := range f.PartialKinds {
		if k == kind {
			complete = false
			if err == "" {
				err = "incomplete"
			}
		}
	}
	return Result{Items: f.Resources[kind], Complete: complete, Error: err, Version: "fixture"}
}
func LoadFixture(path string) (Fixture, error) {
	var f Fixture
	b, err := os.ReadFile(path)
	if err != nil {
		return f, err
	}
	if len(b) > 32*1024*1024 {
		return f, errors.New("fixture exceeds 32 MiB")
	}
	err = json.Unmarshal(b, &f)
	return f, err
}

type Published struct {
	Body     []byte
	Ready    bool
	Revision uint64
}
type Store struct {
	Reader    Reader
	Config    correlate.Config
	Mode      string
	Context   string
	Interval  time.Duration
	Timeout   time.Duration
	mu        sync.Mutex
	raw       map[string][]unstructured.Unstructured
	states    map[string]model.SourceState
	revision  uint64
	published atomic.Pointer[Published]
}

func NewStore(reader Reader, cfg correlate.Config, mode, contextName string, interval, timeout time.Duration) *Store {
	s := &Store{Reader: reader, Config: cfg, Mode: mode, Context: contextName, Interval: interval, Timeout: timeout, raw: map[string][]unstructured.Unstructured{}, states: map[string]model.SourceState{}}
	for _, kind := range Kinds {
		scope := []string{"cluster"}
		if kind == "cells" {
			scope = []string{"all namespaces"}
			if len(cfg.CellNamespaces) > 0 {
				scope = cfg.CellNamespaces
			}
		}
		s.states[kind] = model.SourceState{Kind: kind, State: "loading", Scope: scope}
	}
	s.publish()
	return s
}
func (s *Store) Get() *Published { return s.published.Load() }
func (s *Store) publish() {
	states := []model.SourceState{}
	usable := false
	for _, kind := range Kinds {
		st := s.states[kind]
		states = append(states, st)
		if st.LastSuccessfulListAt != "" || len(s.raw[kind]) > 0 {
			usable = true
		}
	}
	snapshot := correlate.Correlate(s.raw, states, s.Config)
	snapshot.Mode = s.Mode
	snapshot.Context = s.Context
	snapshot.PollSeconds = s.Interval.Seconds()
	snapshot.Revision = s.revision
	snapshot.GeneratedAt = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(snapshot)
	if err != nil {
		panic(fmt.Sprintf("snapshot marshal failed: %v", err))
	}
	s.published.Store(&Published{Body: b, Ready: usable, Revision: s.revision})
}
func (s *Store) Refresh(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	results := make([]Result, len(Kinds))
	var wg sync.WaitGroup
	for i, kind := range Kinds {
		wg.Add(1)
		go func(i int, kind string) {
			defer wg.Done()
			c, cancel := context.WithTimeout(ctx, s.Timeout)
			defer cancel()
			results[i] = s.Reader.Read(c, kind)
		}(i, kind)
	}
	wg.Wait()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i, kind := range Kinds {
		r := results[i]
		valid, rejected := validateObjects(kind, r.Items)
		r.Items = valid
		if rejected {
			r.Complete = false
			r.Error = "incompatible-schema"
		}
		st := s.states[kind]
		st.LastAttemptAt = now
		st.Complete = r.Complete
		st.Error = r.Error
		if r.Complete && r.Error == "" {
			s.raw[kind] = r.Items
			st.State = "ready"
			st.LastSuccessfulListAt = now
			st.ResourceVersion = r.Version
		} else {
			st.State = r.Error
			if st.State == "" {
				st.State = "incomplete"
			}
			if len(r.Items) > 0 {
				st.State = "partial"
				s.raw[kind] = merge(s.raw[kind], r.Items)
				if len(s.raw[kind]) > 10000 {
					s.raw[kind] = s.raw[kind][:10000]
					st.Error = "cache-limit"
				}
			} else if st.LastSuccessfulListAt != "" {
				st.State = "stale"
			}
		}
		st.Count = len(s.raw[kind])
		s.states[kind] = st
	}
	s.revision++
	s.publish()
}
func merge(old, new []unstructured.Unstructured) []unstructured.Unstructured {
	out := make([]unstructured.Unstructured, 0, len(old)+len(new))
	index := map[string]int{}
	for _, o := range old {
		index[o.GetNamespace()+"/"+o.GetName()] = len(out)
		out = append(out, o)
	}
	for _, o := range new {
		key := o.GetNamespace() + "/" + o.GetName()
		if i, ok := index[key]; ok {
			out[i] = o
		} else {
			index[key] = len(out)
			out = append(out, o)
		}
	}
	return out
}
func (s *Store) Run(ctx context.Context) {
	s.Refresh(ctx)
	timer := time.NewTicker(s.Interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.Refresh(ctx)
		}
	}
}

// Reject incompatible typed fields rather than silently publishing zero values.
// Extra upstream fields remain ignored by the private DTO allowlist.
func validateObjects(kind string, items []unstructured.Unstructured) ([]unstructured.Unstructured, bool) {
	out := make([]unstructured.Unstructured, 0, len(items))
	rejected := false
	expected := map[string]string{"switches": "Switch", "nodes": "Node", "interfaces": "Interface", "cells": "Cell"}
	for _, o := range items {
		b, err := json.Marshal(o.Object)
		if err == nil {
			switch kind {
			case "switches":
				var v model.Switch
				err = json.Unmarshal(b, &v)
			case "nodes":
				var v model.Node
				err = json.Unmarshal(b, &v)
			case "interfaces":
				var v model.Interface
				err = json.Unmarshal(b, &v)
			case "cells":
				var v model.Cell
				err = json.Unmarshal(b, &v)
			}
		}
		if err != nil || o.GetName() == "" || o.GetKind() != expected[kind] || o.GetAPIVersion() != GVRs[kind].Group+"/"+GVRs[kind].Version {
			rejected = true
			continue
		}
		out = append(out, *o.DeepCopy())
	}
	return out, rejected
}
