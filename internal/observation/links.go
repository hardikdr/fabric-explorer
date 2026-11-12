package observation

import (
	"sort"
	"strings"
	"time"
)

func fresh[T any](d Domain[T], now time.Time, limit time.Duration) bool {
	return d.State == "ready" && d.Complete && d.SampledAt != nil && !d.SampledAt.After(now.Add(time.Second)) && now.Sub(*d.SampledAt) <= limit
}
func portName(sw Switch, id string) string {
	exact := ""
	for _, p := range sw.Ports.Data {
		if p.Name == id || p.Alias != "" && p.Alias == id {
			if exact != "" && exact != p.Name {
				return ""
			}
			exact = p.Name
		}
	}
	return exact
}
func eligible(sw Switch, now time.Time, limit time.Duration) bool {
	return sw.IdentityState == "verified" && sw.SwitchUID != "" && fresh(sw.Device, now, limit) && fresh(sw.Ports, now, limit) && fresh(sw.LLDP, now, limit)
}
func ResolveLinks(switches []Switch, now time.Time, limit time.Duration) []Link {
	out := []Link{}
	byMAC := map[string][]Switch{}
	for _, sw := range switches {
		if eligible(sw, now, limit) {
			m := mac(sw.Device.Data.MAC)
			if m != "" {
				byMAC[m] = append(byMAC[m], sw)
			}
		}
	}
	seen := map[string]bool{}
	for _, sw := range switches {
		if !eligible(sw, now, limit) {
			continue
		}
		counts := map[string]int{}
		for _, n := range sw.LLDP.Data {
			counts[n.LocalPort]++
		}
		for _, n := range sw.LLDP.Data {
			if counts[n.LocalPort] != 1 || strings.ToLower(n.ChassisSubtype) != "mac" {
				continue
			}
			remote := byMAC[mac(n.ChassisID)]
			if len(remote) != 1 || remote[0].SwitchUID == sw.SwitchUID {
				continue
			}
			other := remote[0]
			if n.TTL != nil && now.Sub(*sw.LLDP.SampledAt) > time.Duration(*n.TTL)*time.Second {
				continue
			}
			if strings.ToLower(n.PortSubtype) != "ifname" && strings.ToLower(n.PortSubtype) != "interface name" && strings.ToLower(n.PortSubtype) != "local" {
				continue
			}
			localPort, remotePort := portName(sw, n.LocalPort), portName(other, n.PortID)
			if localPort == "" || remotePort == "" {
				continue
			}
			reciprocal := false
			remoteMatches := 0
			for _, back := range other.LLDP.Data {
				if portName(other, back.LocalPort) == remotePort {
					remoteMatches++
					if strings.ToLower(back.ChassisSubtype) == "mac" && mac(back.ChassisID) == mac(sw.Device.Data.MAC) && (strings.ToLower(back.PortSubtype) == "ifname" || strings.ToLower(back.PortSubtype) == "interface name" || strings.ToLower(back.PortSubtype) == "local") && portName(sw, back.PortID) == localPort && (back.TTL == nil || now.Sub(*other.LLDP.SampledAt) <= time.Duration(*back.TTL)*time.Second) {
						reciprocal = true
					}
				}
			}
			if remoteMatches > 1 {
				continue
			}
			a, b := sw.SwitchUID+"/"+localPort, other.SwitchUID+"/"+remotePort
			source, target, sourcePort, targetPort := sw.SwitchUID, other.SwitchUID, localPort, remotePort
			if a > b {
				a, b = b, a
				source, target = target, source
				sourcePort, targetPort = targetPort, sourcePort
			}
			id := a + "|" + b
			if seen[id] {
				continue
			}
			seen[id] = true
			evidence := "one-sided LLDP"
			sample := *sw.LLDP.SampledAt
			if reciprocal {
				evidence = "reciprocal LLDP"
				if other.LLDP.SampledAt.Before(sample) {
					sample = *other.LLDP.SampledAt
				}
			}
			out = append(out, Link{Source: source, Target: target, SourcePort: sourcePort, TargetPort: targetPort, Evidence: evidence, SampledAt: sample})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Source+out[i].SourcePort+out[i].Target < out[j].Source+out[j].SourcePort+out[j].Target
	})
	return out
}
