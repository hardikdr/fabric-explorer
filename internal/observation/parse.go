package observation

import (
	"encoding/json"
	"errors"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var splitColumns = regexp.MustCompile(`\s{2,}`)
var macPattern = regexp.MustCompile(`(?i)(?:[0-9a-f]{2}:){5}[0-9a-f]{2}`)

func mac(s string) string {
	parsed, err := net.ParseMAC(s)
	if err != nil || len(parsed) != 6 {
		return ""
	}
	return parsed.String()
}
func text(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return ""
}
func number(v any) *uint64 {
	s := text(v)
	n, e := strconv.ParseUint(s, 10, 64)
	if e != nil {
		return nil
	}
	return &n
}
func state(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "up":
		return "Up"
	case "down":
		return "Down"
	default:
		return "Unknown"
	}
}
func clean(s string) string {
	if len(s) > 512 {
		return s[:512]
	}
	return s
}
func ParseDevice(b []byte) (Device, error) {
	d := Device{}
	for _, line := range strings.Split(string(b), "\n") {
		lower := strings.ToLower(line)
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		v := strings.TrimSpace(parts[1])
		switch {
		case strings.Contains(lower, "mac address"):
			d.MAC = mac(macPattern.FindString(line))
		case strings.HasPrefix(lower, "hostname:"):
			d.Hostname = clean(v)
		case strings.Contains(lower, "software version:"):
			d.Version = clean(v)
		case strings.HasPrefix(lower, "platform:"):
			d.Platform = clean(v)
		}
	}
	if d.MAC == "" {
		return d, errors.New("identity-mac-not-reported")
	}
	return d, nil
}
func ParseEEPROM(b []byte) string {
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(strings.ToLower(line), "base mac") {
			return mac(macPattern.FindString(line))
		}
	}
	return ""
}
func table(b []byte, required string) ([]string, [][]string, error) {
	var headers []string
	rows := [][]string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+-") {
			continue
		}
		cols := splitColumns.Split(line, -1)
		if headers == nil {
			for _, c := range cols {
				if strings.EqualFold(c, required) {
					headers = cols
					break
				}
			}
			continue
		}
		rows = append(rows, cols)
	}
	if headers == nil {
		return nil, nil, errors.New("unsupported-schema")
	}
	return headers, rows, nil
}
func field(headers, row []string, name string) string {
	for i, h := range headers {
		if strings.EqualFold(strings.ReplaceAll(h, " ", ""), strings.ReplaceAll(name, " ", "")) && i < len(row) {
			return strings.TrimSpace(row[i])
		}
	}
	return ""
}
func ParsePorts(b []byte) ([]Port, error) {
	h, rows, e := table(b, "Interface")
	if e != nil {
		return nil, e
	}
	out := []Port{}
	partial := false
	for _, row := range rows {
		n := field(h, row, "Interface")
		if n == "" || len(row) < len(h)-1 {
			partial = true
			continue
		}
		out = append(out, Port{Name: clean(n), Alias: clean(field(h, row, "Alias")), Admin: state(field(h, row, "Admin")), Oper: state(field(h, row, "Oper")), Speed: clean(field(h, row, "Speed")), MTU: number(field(h, row, "MTU")), FEC: clean(firstNonempty(field(h, row, "FEC"), field(h, row, "Oper FEC")))})
	}
	if partial {
		return out, errors.New("partial-parse")
	}
	if len(out) > 5000 {
		return out[:5000], errors.New("object-limit")
	}
	return out, nil
}
func ParseAddresses(b []byte, family string) ([]IPInterface, error) {
	if _, _, e := table(b, "Interface"); e != nil {
		return nil, e
	}
	out := []IPInterface{}
	current := ""
	isAddress := func(v string) bool {
		host := strings.Split(v, "/")[0]
		host = strings.Split(host, "%")[0]
		return net.ParseIP(host) != nil
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] == "Interface" || strings.HasPrefix(fields[0], "---") {
			continue
		}
		if !isAddress(fields[0]) {
			current = fields[0]
		}
		for _, v := range fields {
			if isAddress(v) {
				if current == "" {
					return out, errors.New("partial-parse")
				}
				out = append(out, IPInterface{Name: clean(current), Address: clean(v), Family: family})
				break // Neighbor IP is a different column, not a local address.
			}
		}
	}
	return out, nil
}
func ParseLLDPTable(b []byte) ([]Neighbor, error) {
	h, rows, e := table(b, "LocalPort")
	if e != nil {
		// Some SONiC versions emit a heading-free table with a capability legend and entry count.
		text := string(b)
		if !strings.Contains(text, "Capability codes:") || !strings.Contains(text, "Total entries displayed:") {
			return nil, e
		}
		h = []string{"LocalPort", "RemoteDevice", "RemotePortID", "Capability", "RemoteInterface"}
		rows = nil
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "Capability codes:") || strings.HasPrefix(line, "Total entries displayed:") {
				continue
			}
			rows = append(rows, splitColumns.Split(line, -1))
		}
	}
	out := []Neighbor{}
	for _, row := range rows {
		n := field(h, row, "LocalPort")
		p := field(h, row, "RemotePortID")
		if n == "" || p == "" {
			return out, errors.New("partial-parse")
		}
		out = append(out, Neighbor{LocalPort: clean(n), PortID: clean(p), SystemName: clean(field(h, row, "RemoteDevice"))})
	}
	return out, nil
}
func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func value(v any) string {
	if m := object(v); m != nil {
		return text(m["value"])
	}
	return text(v)
}
func ParseLLDP(b []byte) ([]Neighbor, error) {
	var root map[string]any
	if json.Unmarshal(b, &root) != nil {
		return nil, errors.New("invalid-json")
	}
	lldp := object(root["lldp"])
	raw, ok := lldp["interface"]
	if !ok {
		return nil, errors.New("unsupported-schema")
	}
	out := []Neighbor{}
	partial := false
	var visit func(any)
	visit = func(v any) {
		if a, ok := v.([]any); ok {
			for _, item := range a {
				visit(item)
			}
			return
		}
		for local, entry := range object(v) {
			m := object(entry)
			if m == nil {
				partial = true
				continue
			}
			p := object(m["port"])
			id := object(p["id"])
			n := Neighbor{LocalPort: clean(local), PortID: clean(value(id["value"])), PortSubtype: clean(text(id["type"])), Age: clean(text(m["age"])), TTL: number(value(p["ttl"]))}
			chassis := object(m["chassis"])
			if len(chassis) > 1 {
				// A shared local port with ambiguous chassis cannot establish a graph edge.
				partial = true
				continue
			}
			for name, ch := range chassis {
				c := object(ch)
				cid := object(c["id"])
				n.SystemName = clean(name)
				if reported := value(c["name"]); reported != "" {
					n.SystemName = clean(reported)
				}
				n.ChassisID = clean(value(cid["value"]))
				n.ChassisSubtype = clean(text(cid["type"]))
				break
			}
			if n.PortID != "" {
				out = append(out, n)
			} else {
				partial = true
			}
		}
	}
	visit(raw)
	sort.Slice(out, func(i, j int) bool { return out[i].LocalPort < out[j].LocalPort })
	if partial {
		return out, errors.New("partial-parse")
	}
	return out, nil
}
func ParseVLANs(b []byte) ([]VLAN, error) {
	// SONiC's ASCII grid permits a VLAN ID followed by continuation member rows.
	out := []VLAN{}
	header := false
	current := uint64(0)
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.Contains(line, "|") {
			continue
		}
		cols := strings.Split(line, "|")
		if len(cols) < 5 {
			continue
		}
		for i := range cols {
			cols[i] = strings.TrimSpace(cols[i])
		}
		if strings.EqualFold(cols[1], "VLAN ID") {
			header = true
			continue
		}
		if !header {
			continue
		}
		if n, e := strconv.ParseUint(cols[1], 10, 64); e == nil && n > 0 && n <= 4094 {
			current = n
		} else if cols[1] != "" {
			continue
		}
		if current == 0 {
			continue
		}
		member, tag := "", ""
		if len(cols) > 4 {
			member = cols[3]
			tag = cols[4]
		}
		out = append(out, VLAN{ID: current, Member: clean(member), Tagging: clean(tag)})
	}
	if !header {
		return nil, errors.New("unsupported-schema")
	}
	return out, nil
}

func firstNonempty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
