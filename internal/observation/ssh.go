package observation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const jump = "hardikdr@[2a10:afc0:e01f:1::1]"

// Commands and destination lookup are private allowlists. Nothing originates in HTTP input.
var commands = map[string]string{
	"version":            "show version",
	"identity-eeprom":    "show platform syseeprom",
	"ports":              "show interfaces status",
	"ipv4":               "show ip interfaces",
	"ipv6":               "show ipv6 interfaces",
	"lldp-json":          "lldpctl -f json",
	"lldp-table":         "show lldp table",
	"vlans":              "show vlan brief",
	"bgp-summary-json":   "vtysh -c 'show bgp vrf all ipv6 unicast summary json'",
	"bgp-neighbors-json": "vtysh -c 'show bgp vrf all neighbors json'",
	"bgp-ipv6-text":      "show ipv6 bgp summary",
	"docker-ps":          `docker ps -a --format '{{json .}}'`,
}

type Result struct {
	Output    []byte
	Error     string
	SampledAt time.Time
	Command   string
}
type Reader interface {
	Read(context.Context, Target, string) Result
}
type Runner func(context.Context, []string) ([]byte, []byte, error)
type SSHReader struct {
	Run                          Runner
	PasswordFile, KnownHostsFile string
}

const maxOutput = 4 * 1024 * 1024

type buffer struct {
	bytes.Buffer
	exceeded bool
	limit    int
	cancel   context.CancelFunc
}

func (b *buffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		b.exceeded = true
		b.cancel()
		return 0, errors.New("output-limit")
	}
	return b.Buffer.Write(p)
}
func run(ctx context.Context, args []string) ([]byte, []byte, error) {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(c, "ssh", args...)
	cmd.WaitDelay = 2 * time.Second
	out := &buffer{limit: maxOutput, cancel: cancel}
	stderr := &buffer{limit: 65536, cancel: cancel}
	cmd.Stdout = out
	cmd.Stderr = stderr
	err := cmd.Run()
	if out.exceeded || stderr.exceeded {
		err = errors.New("output-limit")
	}
	return out.Bytes(), stderr.Bytes(), err
}
func validTarget(t Target) bool {
	for _, known := range LabTargets {
		if t == known {
			return true
		}
	}
	return false
}
func (r SSHReader) Read(ctx context.Context, t Target, id string) Result {
	result := Result{Command: id}
	command, ok := commands[id]
	if !ok || !validTarget(t) {
		result.Error = "not-allowlisted"
		return result
	}
	if r.PasswordFile != "" && r.Run == nil {
		return r.passwordRead(ctx, t, id, command)
	}
	fn := r.Run
	if fn == nil {
		fn = run
	}
	out, stderr, err := fn(ctx, []string{"-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes", "-o", "ForwardAgent=no", "-o", "ClearAllForwardings=yes", "-o", "ConnectTimeout=10", "-o", "ServerAliveInterval=5", "-o", "ServerAliveCountMax=2", "-J", jump, "admin@" + t.Address, command})
	if err != nil {
		result.Error = "unavailable"
		msg := strings.ToLower(string(stderr))
		switch {
		case ctx.Err() != nil:
			result.Error = "timeout"
		case err.Error() == "output-limit":
			result.Error = "output-limit"
		case strings.Contains(msg, "host key verification failed"), strings.Contains(msg, "remote host identification has changed"):
			result.Error = "host-key-verification"
		case strings.Contains(msg, "permission denied"):
			result.Error = "authentication"
		case unsupported(msg):
			result.Error = "unsupported-command"
		}
		return result
	}
	if len(out) > maxOutput {
		result.Error = "output-limit"
		return result
	}
	text := strings.ToLower(string(out))
	if unsupported(text) {
		result.Error = "unsupported-command"
		return result
	}
	result.Output = out
	result.SampledAt = time.Now().UTC()
	return result
}

func unsupported(msg string) bool {
	for _, phrase := range []string{"command not found", "unknown command", "invalid choice", "no such command", "not found"} {
		if strings.Contains(msg, phrase) {
			return true
		}
	}
	return false
}

// Capture replay never claims a new device sample time. Each command retains its capture timestamp.
type Capture struct {
	Provenance string `json:"provenance"`
	Targets    map[string]map[string]struct {
		Output    string    `json:"output"`
		Error     string    `json:"error,omitempty"`
		SampledAt time.Time `json:"sampledAt"`
	} `json:"targets"`
}
type Replay struct{ Path string }

func (r Replay) Read(_ context.Context, t Target, id string) Result {
	result := Result{Command: id}
	file, e := os.Open(r.Path)
	if e != nil {
		result.Error = "capture-unavailable"
		return result
	}
	defer file.Close()
	data, e := io.ReadAll(io.LimitReader(file, 32*1024*1024+1))
	if e != nil || len(data) > 32*1024*1024 {
		result.Error = "invalid-capture"
		return result
	}
	var capture Capture
	if json.Unmarshal(data, &capture) != nil {
		result.Error = "invalid-capture"
		return result
	}
	sample, ok := capture.Targets[t.ResourceName][id]
	if !ok || sample.SampledAt.IsZero() {
		result.Error = "capture-missing"
		return result
	}
	if time.Since(sample.SampledAt) < -time.Second {
		result.Error = "invalid-capture-time"
		return result
	}
	result.Output = []byte(sample.Output)
	result.Error = sample.Error
	result.SampledAt = sample.SampledAt
	return result
}
