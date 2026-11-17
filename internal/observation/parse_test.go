package observation

import (
	"testing"
)

func TestSyntheticParserFailuresAndUnknownStates(t *testing.T) {
	if _, e := ParsePorts([]byte("command failed")); e == nil {
		t.Fatal("false empty inventory")
	}
	ports, e := ParsePorts([]byte("Interface  Speed  MTU  FEC  Alias  Oper  Admin\nEthernet0  100G  9100  rs  etp1  mystery  down\n"))
	if e != nil || len(ports) != 1 || ports[0].Oper != "Unknown" || ports[0].Admin != "Down" {
		t.Fatal(ports, e)
	}
	sessions, e := ParseBGP([]byte(`{"blue":{"ipv6Unicast":{"peers":{"Ethernet0":{"remoteAs":4200000000,"state":"Idle"}}}}}`))
	if e != nil || len(sessions) != 1 || sessions[0].State != "Idle" || sessions[0].VRF != "blue" {
		t.Fatal(sessions, e)
	}
	if _, e = ParseBGP([]byte("{}")); e == nil {
		t.Fatal("unrecognized JSON treated as empty")
	}
	if _, e = ParseLLDP([]byte("{}")); e == nil {
		t.Fatal("unrecognized LLDP treated as empty")
	}
	table, e := ParseLLDPTable([]byte("LocalPort  RemoteDevice  RemotePortID  Capability\nEthernet0  peer  Ethernet4  BR\n"))
	if e != nil || table[0].ChassisID != "" {
		t.Fatal(table, e)
	}
}

func TestParseContainers(t *testing.T) {
	input := `{"ID":"24d51a09a9a1","Names":"telemetry","Image":"docker-sonic-telemetry:latest","State":"running","Status":"Up 6 days","CreatedAt":"2026-11-10 12:35:04 +0000 UTC","RunningFor":"6 days ago"}
{"ID":"6d86073abfd8","Names":"fabriclet-sonic","Image":"ghcr.io/hardikdr/fabriclet-sonic:configdb-bgp-v2","State":"running","Status":"Up 6 days","CreatedAt":"2026-11-10 12:31:43 +0000 UTC","RunningFor":"6 days ago"}
`
	containers, err := ParseContainers([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(containers))
	}
	if containers[0].ID != "24d51a09a9a1" || containers[0].Name != "telemetry" {
		t.Fatalf("unexpected first container: %+v", containers[0])
	}
	if containers[1].Name != "fabriclet-sonic" || containers[1].Image != "ghcr.io/hardikdr/fabriclet-sonic:configdb-bgp-v2" {
		t.Fatalf("unexpected second container: %+v", containers[1])
	}
	if containers[0].State != "running" {
		t.Fatalf("expected state running, got %q", containers[0].State)
	}
}

func TestParseContainersEmpty(t *testing.T) {
	containers, err := ParseContainers([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error on empty input: %v", err)
	}
	if len(containers) != 0 {
		t.Fatalf("expected empty, got %d", len(containers))
	}
}

func TestParseContainersMalformed(t *testing.T) {
	// Malformed lines are skipped; valid ones are kept.
	input := `not-json
{"ID":"abc123","Names":"good","Image":"img","State":"running","Status":"Up","CreatedAt":"now","RunningFor":"1h"}
{broken`
	containers, _ := ParseContainers([]byte(input))
	if len(containers) != 1 || containers[0].ID != "abc123" {
		t.Fatalf("expected 1 valid container, got %d: %+v", len(containers), containers)
	}
}

