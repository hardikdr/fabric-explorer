package observation

import (
	"testing"
	"time"
)

func observedPair(now time.Time) []Switch {
	a, b := Switch{Target: Target{ResourceName: "leaf-1", ExpectedMAC: "02:00:00:00:01:01"}, SwitchUID: "a", IdentityState: "verified"}, Switch{Target: Target{ResourceName: "spine-1", ExpectedMAC: "02:00:00:00:02:01"}, SwitchUID: "b", IdentityState: "verified"}
	a.Device = Domain[Device]{State: "ready", Complete: true, SampledAt: &now, Data: Device{MAC: a.Target.ExpectedMAC}}
	b.Device = Domain[Device]{State: "ready", Complete: true, SampledAt: &now, Data: Device{MAC: b.Target.ExpectedMAC}}
	a.Ports = Domain[[]Port]{State: "ready", Complete: true, SampledAt: &now, Data: []Port{{Name: "Ethernet120", Alias: "Eth31"}}}
	b.Ports = Domain[[]Port]{State: "ready", Complete: true, SampledAt: &now, Data: []Port{{Name: "Ethernet0", Alias: "Eth1"}}}
	a.LLDP = Domain[[]Neighbor]{State: "ready", Complete: true, SampledAt: &now, Data: []Neighbor{{LocalPort: "Ethernet120", ChassisID: b.Device.Data.MAC, ChassisSubtype: "mac", PortID: "Eth1", PortSubtype: "local"}}}
	b.LLDP = Domain[[]Neighbor]{State: "ready", Complete: true, SampledAt: &now, Data: []Neighbor{{LocalPort: "Ethernet0", ChassisID: a.Device.Data.MAC, ChassisSubtype: "mac", PortID: "Eth31", PortSubtype: "local"}}}
	return []Switch{a, b}
}
func TestObservedLinksEvidence(t *testing.T) {
	now := time.Now()
	limit := 90 * time.Second
	pair := observedPair(now)
	links := ResolveLinks(pair, now, limit)
	if len(links) != 1 || links[0].Evidence != "reciprocal LLDP" || links[0].SourcePort != "Ethernet120" {
		t.Fatal(links)
	}
	pair[1].LLDP.Data = nil
	links = ResolveLinks(pair, now, limit)
	if len(links) != 1 || links[0].Evidence != "one-sided LLDP" {
		t.Fatal(links)
	}
	for _, mutate := range []func([]Switch){
		func(p []Switch) { p[0].IdentityState = "mismatch" },
		func(p []Switch) { p[1].Ports.State = "stale" },
		func(p []Switch) {
			p[1].Ports.Data = append(p[1].Ports.Data, Port{Name: "Ethernet4", Alias: "Eth1"})
			p[1].LLDP.Data = nil
		},
		func(p []Switch) { p[0].LLDP.Data = append(p[0].LLDP.Data, p[0].LLDP.Data[0]) },
		func(p []Switch) { p[0].LLDP.Data[0].PortSubtype = "description"; p[1].LLDP.Data = nil },
		func(p []Switch) { p[0].LLDP.Data[0].ChassisSubtype = "local"; p[1].LLDP.Data = nil },
	} {
		p := observedPair(now)
		mutate(p)
		if len(ResolveLinks(p, now, limit)) != 0 {
			t.Fatal("insufficient evidence drew link")
		}
	}
	if len(ResolveLinks(observedPair(now), now.Add(91*time.Second), limit)) != 0 {
		t.Fatal("expired observations drew links")
	}
	p := observedPair(now)
	ttl := uint64(1)
	p[0].LLDP.Data[0].TTL = &ttl
	p[1].LLDP.Data[0].TTL = &ttl
	if len(ResolveLinks(p, now.Add(2*time.Second), limit)) != 0 {
		t.Fatal("TTL ignored")
	}
}
