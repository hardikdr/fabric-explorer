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
