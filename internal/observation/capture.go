package observation

import (
	"context"
	"encoding/json"
	"regexp"

	"strings"
	"time"
)

func CommandIDs() []string {
	return []string{"version", "identity-eeprom", "ports", "ipv4", "ipv6", "lldp-table", "lldp-json", "vlans", "bgp-summary-json", "bgp-neighbors-json", "bgp-ipv6-text"}
}
func Sanitize(id string, b []byte) string {
	if id == "version" {
		lines := []string{}
		for _, l := range strings.Split(string(b), "\n") {
			lower := strings.ToLower(strings.TrimSpace(l))
			for _, prefix := range []string{"sonic software version:", "platform:", "hostname:", "base mac address:", "mac address:"} {
				if strings.HasPrefix(lower, prefix) {
					lines = append(lines, l)
					break
				}
			}
		}
		return strings.Join(lines, "\n") + "\n"
	}
	if id == "identity-eeprom" {
		lines := []string{}
		for _, l := range strings.Split(string(b), "\n") {
			if strings.Contains(strings.ToLower(l), "base mac") {
				lines = append(lines, l)
			}
		}
		return strings.Join(lines, "\n") + "\n"
	}
	var v any
	if json.Unmarshal(b, &v) == nil {
		var redact func(any) any
		redact = func(v any) any {
			switch x := v.(type) {
			case map[string]any:
				out := map[string]any{}
				for k, value := range x {
					if !sensitive.MatchString(k) {
						out[k] = redact(value)
					}
				}
				return out
			case []any:
				for i := range x {
					x[i] = redact(x[i])
				}
				return x
			}
			return v
		}
		out, _ := json.MarshalIndent(redact(v), "", "  ")
		return string(out) + "\n"
	}
	lines := []string{}
	for _, line := range strings.Split(string(b), "\n") {
		if !sensitive.MatchString(line) {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

var sensitive = regexp.MustCompile(`(?i)password|secret|credential|private.?key|serial|inventory|token|descr`)

func CaptureTarget(ctx context.Context, reader Reader, t Target, timeout time.Duration) map[string]Result {
	out := map[string]Result{}
	for _, id := range CommandIDs() {
		c, cancel := context.WithTimeout(ctx, timeout)
		r := reader.Read(c, t, id)
		cancel()
		r.Output = []byte(Sanitize(id, r.Output))
		out[id] = r
		if r.Error == "host-key-verification" || r.Error == "authentication" || r.Error == "ssh-handshake-failed" {
			for _, remaining := range CommandIDs() {
				if _, ok := out[remaining]; !ok {
					out[remaining] = Result{Command: remaining, Error: "not-attempted"}
				}
			}
			break
		}
	}
	return out
}
