package model

// These are private, allowlisted read DTOs, not public Kubernetes API definitions.
type Identity struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace,omitempty"`
	UID               string            `json:"uid"`
	Generation        int64             `json:"generation"`
	ResourceVersion   string            `json:"resourceVersion"`
	Labels            map[string]string `json:"labels,omitempty"`
	DeletionTimestamp string            `json:"deletionTimestamp,omitempty"`
}

func (i Identity) Key() string {
	if i.UID != "" {
		return i.UID
	}
	return i.Namespace + "/" + i.Name
}

type Condition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	Reason             string `json:"reason"`
	Message            string `json:"message"`
	LastTransitionTime string `json:"lastTransitionTime"`
	ObservedGeneration *int64 `json:"observedGeneration"`
}
type Ref struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	UID       string `json:"uid,omitempty"`
}
type Resource[S any, T any] struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   Identity `json:"metadata"`
	Spec       S        `json:"spec"`
	Status     T        `json:"status"`
}
type Status struct {
	State           string      `json:"state,omitempty"`
	Phase           string      `json:"phase,omitempty"`
	OperationState  string      `json:"operationState,omitempty"`
	Conditions      []Condition `json:"conditions"`
	MACAddress      string      `json:"macAddress,omitempty"`
	FirmwareVersion string      `json:"firmwareVersion,omitempty"`
	SKU             string      `json:"sku,omitempty"`
}
type Management struct {
	Host string `json:"host"`
	Port string `json:"port"`
}
type PortSpec struct {
	Name string `json:"name"`
}
type SwitchSpec struct {
	Hostname     string     `json:"hostname"`
	MACAddress   string     `json:"macAddress"`
	Management   Management `json:"management"`
	Ports        []PortSpec `json:"ports"`
	NextBootMode string     `json:"nextBootMode,omitempty"`
}
type NodeSpec struct {
	ProviderID string `json:"providerID"`
	CellRef    *Ref   `json:"cellRef,omitempty"`
}
type InterfaceSpec struct {
	NodeRef    Ref    `json:"nodeRef"`
	Handle     string `json:"handle"`
	AdminState string `json:"adminState"`
}
type Peer struct {
	InterfaceRef Ref    `json:"interfaceRef"`
	DHCPRelay    string `json:"dhcpRelay,omitempty"`
}
type CellSpec struct {
	NodeRef  Ref      `json:"nodeRef"`
	ID       string   `json:"id"`
	Hostname string   `json:"hostname"`
	IPs      []string `json:"ips"`
	Prefixes []string `json:"prefixes"`
	Peers    []Peer   `json:"peers"`
}
type Switch = Resource[SwitchSpec, Status]
type Node = Resource[NodeSpec, Status]
type Interface = Resource[InterfaceSpec, Status]
type Cell = Resource[CellSpec, Status]
type Diagnostic struct {
	Code     string   `json:"code"`
	Severity string   `json:"severity"`
	Summary  string   `json:"summary"`
	Related  []string `json:"related"`
}
type Evidence struct {
	Type     string `json:"type"`
	NodeName string `json:"nodeName"`
	Detail   string `json:"detail"`
}
type Association struct {
	State      string     `json:"state"`
	NodeKey    string     `json:"nodeKey,omitempty"`
	Candidates []string   `json:"candidates"`
	Evidence   []Evidence `json:"evidence"`
}
type PeerView struct {
	Peer
	Purpose      string `json:"purpose"`
	Prefix       string `json:"prefix,omitempty"`
	InterfaceKey string `json:"interfaceKey,omitempty"`
	Resolution   string `json:"resolution"`
}
type CellView struct {
	Resource    Cell         `json:"resource"`
	Binding     string       `json:"binding"`
	Role        string       `json:"role"`
	Peers       []PeerView   `json:"peers"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}
type NodeView struct {
	Resource      Node         `json:"resource"`
	BoundCellKey  string       `json:"boundCellKey,omitempty"`
	Binding       string       `json:"binding"`
	CellKeys      []string     `json:"cellKeys"`
	InterfaceKeys []string     `json:"interfaceKeys"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
}
type DeclaredPort struct {
	Name         string `json:"name"`
	InterfaceKey string `json:"interfaceKey,omitempty"`
	Resolution   string `json:"resolution"`
}
type SwitchView struct {
	Resource      Switch         `json:"resource"`
	DisplayName   string         `json:"displayName"`
	Association   Association    `json:"association"`
	Role          string         `json:"role"`
	RoleCellKey   string         `json:"roleCellKey,omitempty"`
	DeclaredPorts []DeclaredPort `json:"declaredPorts"`
	Diagnostics   []Diagnostic   `json:"diagnostics"`
}
type SourceState struct {
	Kind                 string   `json:"kind"`
	State                string   `json:"state"`
	Complete             bool     `json:"complete"`
	Count                int      `json:"count"`
	LastAttemptAt        string   `json:"lastAttemptAt"`
	LastSuccessfulListAt string   `json:"lastSuccessfulListAt,omitempty"`
	ResourceVersion      string   `json:"resourceVersion,omitempty"`
	Error                string   `json:"error,omitempty"`
	Scope                []string `json:"scope"`
}
type Snapshot struct {
	SchemaVersion string              `json:"schemaVersion"`
	Revision      uint64              `json:"revision"`
	GeneratedAt   string              `json:"generatedAt"`
	Mode          string              `json:"mode"`
	Context       string              `json:"context"`
	PollSeconds   float64             `json:"pollSeconds"`
	Sources       []SourceState       `json:"sourceStates"`
	Switches      []SwitchView        `json:"switches"`
	Nodes         []NodeView          `json:"nodes"`
	Cells         []CellView          `json:"cells"`
	Interfaces    []Interface         `json:"interfaces"`
	Unmatched     map[string][]string `json:"unmatched"`
	Diagnostics   []Diagnostic        `json:"diagnostics"`
}
