package source

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const LabTarget = "hardikdr@2a10:afc0:e01f:ffff:5054:ff:fee4:7ae9"
const LabJump = "hardikdr@[2a10:afc0:e01f:1::1]"
const maxSSHBytes = 32 * 1024 * 1024

// Only these fixed commands are accepted. Browser input never reaches SSH.
var labCommands = map[string]string{
	"switches":   "kubectl get switches.sonic.networking.metal.ironcore.dev -o json --chunk-size=500 --request-timeout=12s",
	"nodes":      "kubectl get nodes.fabric.ironcore.dev -o json --chunk-size=500 --request-timeout=12s",
	"interfaces": "kubectl get interfaces.fabric.ironcore.dev -o json --chunk-size=500 --request-timeout=12s",
	"cells":      "kubectl get cells.fabric.ironcore.dev -A -o json --chunk-size=500 --request-timeout=12s",
}

type SSHRunner func(context.Context, []string) ([]byte, []byte, error)
type SSHReader struct {
	Namespaces []string // Local filtering only; never shell-interpolated.
	Run        SSHRunner
	once       sync.Once
	slots      chan struct{}
}
type boundedBuffer struct {
	bytes.Buffer
	max int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.max-b.Len() {
		return 0, errors.New("output-limit")
	}
	return b.Buffer.Write(p)
}

type cancelWriter struct {
	writer io.Writer
	cancel context.CancelFunc
}

func (w cancelWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if err != nil {
		w.cancel()
	}
	return n, err
}
func runSSH(ctx context.Context, args []string) ([]byte, []byte, error) {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(c, "ssh", args...)
	cmd.WaitDelay = 2 * time.Second
	out := &boundedBuffer{max: maxSSHBytes}
	stderr := &boundedBuffer{max: 64 * 1024}
	cmd.Stdout = cancelWriter{out, cancel}
	cmd.Stderr = cancelWriter{stderr, cancel}
	err := cmd.Run()
	return out.Bytes(), stderr.Bytes(), err
}
func (r *SSHReader) Read(ctx context.Context, kind string) Result {
	command, ok := labCommands[kind]
	if !ok {
		return Result{Error: "unsupported-source"}
	}
	r.once.Do(func() { r.slots = make(chan struct{}, 2) })
	select {
	case r.slots <- struct{}{}:
	case <-ctx.Done():
		return Result{Error: "timeout"}
	}
	defer func() { <-r.slots }()
	run := r.Run
	if run == nil {
		run = runSSH
	}
	args := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-o", "StrictHostKeyChecking=yes", "-o", "ForwardAgent=no", "-o", "ClearAllForwardings=yes", "-o", "ServerAliveInterval=5", "-o", "ServerAliveCountMax=2", "-J", LabJump, LabTarget, command}
	out, stderr, err := run(ctx, args)
	if err != nil {
		code := "unavailable"
		message := strings.ToLower(string(stderr))
		switch {
		case ctx.Err() != nil:
			code = "timeout"
		case strings.Contains(message, "forbidden"):
			code = "forbidden"
		case strings.Contains(message, "doesn't have a resource type"), strings.Contains(message, "could not find the requested resource"):
			code = "missing-crd"
		case strings.Contains(message, "host key verification failed"), strings.Contains(message, "remote host identification has changed"):
			code = "ssh-host-verification"
		case strings.Contains(message, "permission denied"):
			code = "ssh-authentication"
		}
		return Result{Error: code} // Never publish raw transport output.
	}
	if len(out) > maxSSHBytes {
		return Result{Error: "output-limit"}
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(out, &envelope) != nil || len(envelope["items"]) == 0 || string(envelope["items"]) == "null" {
		return Result{Error: "invalid-json"}
	}
	var list unstructured.UnstructuredList
	if json.Unmarshal(out, &list) != nil || list.GetKind() != "List" {
		return Result{Error: "invalid-json"}
	}
	res := Result{Items: list.Items, Complete: true, Version: list.GetResourceVersion()}
	if len(res.Items) > 10000 {
		res.Items = res.Items[:10000]
		res.Complete = false
		res.Error = "object-limit"
	}
	if list.GetContinue() != "" {
		res.Complete = false
		res.Error = "incomplete"
	}
	if kind == "cells" && len(r.Namespaces) > 0 {
		allowed := map[string]bool{}
		for _, ns := range r.Namespaces {
			allowed[ns] = true
		}
		filtered := []unstructured.Unstructured{}
		for _, item := range res.Items {
			if allowed[item.GetNamespace()] {
				filtered = append(filtered, item)
			}
		}
		res.Items = filtered
	}
	return res
}
