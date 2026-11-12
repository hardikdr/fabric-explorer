// Package observation contains sampled operational state, separate from Kubernetes DTOs.
package observation

import "time"

type Target struct {
	ResourceName     string `json:"resourceName"`
	Address          string `json:"address"`
	ExpectedMAC      string `json:"expectedMAC"`
	ExpectedHostname string `json:"expectedHostname,omitempty"`
}

type Device struct {
	MAC      string `json:"mac"`
	Hostname string `json:"hostname,omitempty"`
	Version  string `json:"version,omitempty"`
	Platform string `json:"platform,omitempty"`
}
type Port struct {
	Name  string  `json:"name"`
	Alias string  `json:"alias,omitempty"`
	Admin string  `json:"admin"`
	Oper  string  `json:"oper"`
	Speed string  `json:"speed,omitempty"`
	MTU   *uint64 `json:"mtu,omitempty"`
	FEC   string  `json:"fec,omitempty"`
}
type IPInterface struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Family  string `json:"family"`
}
type BGP struct {
	Neighbor       string  `json:"neighbor"`
	VRF            string  `json:"vrf"`
	AF             string  `json:"addressFamily"`
	LocalInterface string  `json:"localInterface,omitempty"`
	RemoteASN      *uint64 `json:"remoteASN,omitempty"`
	State          string  `json:"state"`
	Uptime         string  `json:"uptime,omitempty"`
	Received       *uint64 `json:"receivedPrefixes,omitempty"`
	Advertised     *uint64 `json:"advertisedPrefixes,omitempty"`
}
type Neighbor struct {
	LocalPort      string  `json:"localPort"`
	ChassisID      string  `json:"chassisID,omitempty"`
	ChassisSubtype string  `json:"chassisSubtype,omitempty"`
	PortID         string  `json:"portID"`
	PortSubtype    string  `json:"portSubtype,omitempty"`
	SystemName     string  `json:"systemName,omitempty"`
	Age            string  `json:"reportedAge,omitempty"`
	TTL            *uint64 `json:"advertisedTTLSeconds,omitempty"`
}
type VLAN struct {
	ID      uint64 `json:"id"`
	Member  string `json:"member,omitempty"`
	Tagging string `json:"tagging,omitempty"`
	// CLI VLAN membership is configuration, not measured forwarding state.
}
type Domain[T any] struct {
	State         string     `json:"state"`
	Complete      bool       `json:"complete"`
	SampledAt     *time.Time `json:"sampledAt,omitempty"`
	LastAttemptAt *time.Time `json:"lastAttemptAt,omitempty"`
	Error         string     `json:"error,omitempty"`
	Command       string     `json:"command,omitempty"`
	Data          T          `json:"data"`
}
type Switch struct {
	Target           Target                `json:"target"`
	SwitchUID        string                `json:"switchUID,omitempty"`
	IdentityState    string                `json:"identityState"`
	IdentityEvidence string                `json:"identityEvidence"`
	Device           Domain[Device]        `json:"device"`
	Ports            Domain[[]Port]        `json:"ports"`
	Addresses        Domain[[]IPInterface] `json:"addresses"`
	BGP              Domain[[]BGP]         `json:"bgp"`
	BGPDetails       Domain[[]BGP]         `json:"bgpDetails"`
	LLDP             Domain[[]Neighbor]    `json:"lldp"`
	VLANs            Domain[[]VLAN]        `json:"vlans"`
}
type Link struct {
	Source     string    `json:"source"`
	Target     string    `json:"target"`
	SourcePort string    `json:"sourcePort"`
	TargetPort string    `json:"targetPort"`
	Evidence   string    `json:"evidence"`
	SampledAt  time.Time `json:"sampledAt"`
}
type Snapshot struct {
	SchemaVersion     string    `json:"schemaVersion"`
	Mode              string    `json:"mode"`
	PollSeconds       float64   `json:"pollSeconds"`
	StaleAfterSeconds float64   `json:"staleAfterSeconds"`
	GeneratedAt       time.Time `json:"generatedAt"`
	Switches          []Switch  `json:"switches"`
	Links             []Link    `json:"links"`
}
