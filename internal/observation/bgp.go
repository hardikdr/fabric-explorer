package observation

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
)

func first(m map[string]any, fields ...string) any {
	for _, f := range fields {
		if v, ok := m[f]; ok {
			return v
		}
	}
	return nil
}
func family(s string) string {
	switch strings.ToLower(strings.ReplaceAll(s, " ", "")) {
	case "ipv6unicast":
		return "ipv6-unicast"
	case "ipv4unicast":
		return "ipv4-unicast"
	}
	return "unknown"
}
func ParseBGP(b []byte) ([]BGP, error) {
	var root any
	decoder := json.NewDecoder(strings.NewReader(string(b)))
	decoder.UseNumber()
	if decoder.Decode(&root) != nil {
		return nil, errors.New("invalid-json")
	}
	out := []BGP{}
	recognized := false
	var walk func(any, string, string, string)
	walk = func(v any, vrf, af, name string) {
		m := object(v)
		if m == nil {
			return
		}
		if s := text(m["vrfName"]); s != "" {
			vrf = s
		}
		if peers, ok := m["peers"]; ok {
			recognized = true
			for neighbor, p := range object(peers) {
				walk(p, vrf, af, neighbor)
			}
			return
		}
		if first(m, "remoteAs", "remoteAS", "remoteASN", "bgpState", "state") != nil && name != "" {
			recognized = true
			s := text(first(m, "bgpState", "state"))
			if s == "" {
				s = "Unknown"
			}
			base := BGP{Neighbor: clean(name), VRF: clean(vrf), AF: af, RemoteASN: number(first(m, "remoteAs", "remoteAS", "remoteASN")), State: clean(s), LocalInterface: clean(text(first(m, "localInterface", "interfaceName", "peerInterface"))), Uptime: clean(text(first(m, "peerUptime", "bgpTimerUpString", "uptime"))), Received: number(first(m, "pfxRcd", "acceptedPrefixCounter")), Advertised: number(first(m, "pfxSnt", "sentPrefixCounter"))}
			if text(m["idType"]) == "interface" && base.LocalInterface == "" {
				base.LocalInterface = clean(name)
			}
			afi := object(m["addressFamilyInfo"])
			if len(afi) > 0 {
				for fam, data := range afi {
					session := base
					session.AF = family(fam)
					d := object(data)
					session.Received = number(first(d, "acceptedPrefixCounter", "pfxRcd"))
					session.Advertised = number(first(d, "sentPrefixCounter", "pfxSnt"))
					out = append(out, session)
				}
			} else {
				out = append(out, base)
			}
			return
		}
		for k, child := range m {
			nextVRF, nextAF := vrf, af
			if f := family(k); f != "unknown" {
				nextAF = f
			} else if name == "" && af == "unknown" && k != "default" && object(child) != nil && first(object(child), "vrfId", "routerId", "ipv6Unicast", "ipv4Unicast") == nil {
				nextVRF = vrf
			} else if k == "default" || first(object(child), "vrfId", "routerId", "ipv6Unicast", "ipv4Unicast") != nil {
				nextVRF = k
			}
			walk(child, nextVRF, nextAF, k)
		}
	}
	walk(root, "default", "unknown", "")
	if !recognized {
		return nil, errors.New("unsupported-schema")
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].VRF+out[i].AF+out[i].Neighbor < out[j].VRF+out[j].AF+out[j].Neighbor
	})
	if len(out) > 10000 {
		return out[:10000], errors.New("object-limit")
	}
	return out, nil
}
func ParseBGPText(b []byte) ([]BGP, error) {
	out := []BGP{}
	header := false
	vrf := "default"
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "BGP summary information for VRF ") {
			vrf = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "BGP summary information for VRF "))
		}
		if strings.Contains(line, "Neighbor") && strings.Contains(line, "State/PfxRcd") {
			header = true
			continue
		}
		if !header {
			continue
		}
		cols := strings.Fields(line)
		if len(cols) < 10 {
			continue
		}
		asn := number(cols[2])
		if asn == nil {
			continue
		}
		last := cols[9]
		s := last
		var count *uint64
		if n, e := strconv.ParseUint(last, 10, 64); e == nil {
			s = "Established"
			count = &n
		} // FRR summary defines numeric State/PfxRcd as Established, not a Cell inference.
		out = append(out, BGP{Neighbor: clean(cols[0]), VRF: clean(vrf), AF: "ipv6-unicast", RemoteASN: asn, State: clean(s), Uptime: clean(cols[8]), Received: count})
	}
	if !header {
		return nil, errors.New("unsupported-schema")
	}
	return out, nil
}
func mergeBGP(summary, details []BGP) []BGP {
	out := append([]BGP{}, summary...)
	for _, d := range details {
		found := false
		for i, s := range out {
			if d.Neighbor == s.Neighbor && d.VRF == s.VRF && (d.AF == s.AF || d.AF == "unknown") {
				found = true
				if d.LocalInterface != "" {
					out[i].LocalInterface = d.LocalInterface
				}
				if d.Advertised != nil {
					out[i].Advertised = d.Advertised
				}
				if d.RemoteASN != nil {
					out[i].RemoteASN = d.RemoteASN
				}
				if d.Uptime != "" {
					out[i].Uptime = d.Uptime
				}
			}
		}
		if !found {
			out = append(out, d)
		}
	}
	return out
}

// The validated summary command explicitly selects IPv6 unicast even when FRR
// does not repeat the address-family name in its JSON envelope.
func ParseBGPSummary(b []byte) ([]BGP, error) {
	out, e := ParseBGP(b)
	for i := range out {
		if out[i].AF == "unknown" {
			out[i].AF = "ipv6-unicast"
		}
	}
	return out, e
}
