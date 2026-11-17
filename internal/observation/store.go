package observation

import (
	"context"
	"encoding/json"
	"github.com/ironcore-dev/fabric-explorer/internal/model"
	"github.com/ironcore-dev/fabric-explorer/internal/source"
	"sync"
	"time"
)

type Store struct {
	Reader                                              Reader
	Inventory                                           *source.Store
	Interval, CommandTimeout, SwitchTimeout, StaleAfter time.Duration
	Mode                                                string
	mu                                                  sync.RWMutex
	switches                                            []Switch
}

func NewStore(reader Reader, inventory *source.Store, mode string, interval, commandTimeout, switchTimeout time.Duration) *Store {
	s := &Store{Reader: reader, Inventory: inventory, Mode: mode, Interval: interval, CommandTimeout: commandTimeout, SwitchTimeout: switchTimeout, StaleAfter: 3 * interval}
	if reader != nil {
		for _, t := range LabTargets {
			s.switches = append(s.switches, Switch{Target: t, IdentityState: "unavailable", IdentityEvidence: "Not yet sampled"})
		}
	}
	return s
}
func update[T any](old Domain[T], r Result, data T, e error) Domain[T] {
	now := time.Now().UTC()
	old.LastAttemptAt = &now
	old.Command = r.Command
	code := r.Error
	if e != nil && code == "" {
		code = e.Error()
	}
	old.Error = code
	old.Complete = code == ""
	if code == "" {
		old.State = "ready"
		old.Data = data
		old.SampledAt = &r.SampledAt
	} else if code == "partial-parse" || code == "object-limit" { // Partial results never overwrite last-good samples.
		if old.SampledAt != nil {
			old.State = "stale"
		} else {
			old.State = "partial"
			old.Data = data
			old.SampledAt = &r.SampledAt
		}
	} else if old.SampledAt != nil {
		old.State = "stale"
	} else {
		old.State = "unavailable"
	}
	return old
}
func mark[T any](d Domain[T], code string) Domain[T] {
	r := Result{Error: code, Command: d.Command}
	return update(d, r, d.Data, nil)
}
func (s *Store) command(ctx context.Context, t Target, id string) Result {
	c, cancel := context.WithTimeout(ctx, s.CommandTimeout)
	defer cancel()
	return s.Reader.Read(c, t, id)
}

// commandVia runs a command on an already-open switch connection. Falls back to
// opening a new connection per command (Reader.Read) when conn is nil.
func (s *Store) commandVia(ctx context.Context, conn *switchConn, t Target, id string) Result {
	c, cancel := context.WithTimeout(ctx, s.CommandTimeout)
	defer cancel()
	if conn != nil {
		cmd, ok := commands[id]
		if !ok {
			return Result{Command: id, Error: "not-allowlisted"}
		}
		return runOn(c, conn.client, id, cmd)
	}
	return s.Reader.Read(c, t, id)
}
func (s *Store) Refresh(ctx context.Context) {
	if s.Reader == nil {
		return
	}
	s.mu.RLock()
	previous := append([]Switch{}, s.switches...)
	s.mu.RUnlock()
	var inv model.Snapshot
	_ = json.Unmarshal(s.Inventory.Get().Body, &inv)
	complete := false
	for _, st := range inv.Sources {
		if st.Kind == "switches" {
			complete = st.State == "ready" && st.Complete
		}
	}
	results := make([]Switch, len(previous))
	slots := make(chan struct{}, 2)
	var wg sync.WaitGroup
	for i, old := range previous {
		wg.Add(1)
		go func(i int, old Switch) {
			defer wg.Done()
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				results[i] = old
				return
			}
			defer func() { <-slots }()
			c, cancel := context.WithTimeout(ctx, s.SwitchTimeout)
			defer cancel()
			previousUID := old.SwitchUID
			old.IdentityState = "unavailable"
			old.SwitchUID = ""
			old.IdentityEvidence = "No current verified device identity"
			var sw *model.Switch
			for _, v := range inv.Switches {
				if v.Resource.Metadata.Name == old.Target.ResourceName {
					copy := v.Resource
					sw = &copy
					break
				}
			}
			block := ""
			if !complete || sw == nil {
				block = "inventory-unavailable"
			} else if mac(sw.Spec.MACAddress) == "" || mac(sw.Spec.MACAddress) != mac(old.Target.ExpectedMAC) {
				block = "inventory-identity-mismatch"
			}
			if block != "" {
				old.IdentityState = block
				old.IdentityEvidence = "Expected MAC conflicts with, or cannot be checked against, current Switch inventory"
				old.Device = mark(old.Device, block)
				degrade(&old, block)
				results[i] = old
				return
			}
			if previousUID != "" && previousUID != sw.Metadata.Key() {
				old = Switch{Target: old.Target, IdentityState: "unavailable"}
			}
			if previousUID == sw.Metadata.Key() {
				old.SwitchUID = previousUID
			}
			// Open one SSH connection for all commands on this switch.
			var conn *switchConn
			if sr, ok := s.Reader.(SSHReader); ok && sr.PasswordFile != "" {
				var errCode string
				conn, errCode = sr.connect(c, old.Target)
				if errCode != "" {
					old.IdentityEvidence = "Device MAC was not verified during this attempt: " + errCode
					if previousUID != "" && previousUID == sw.Metadata.Key() {
						old.SwitchUID = previousUID
					}
					old.Device = mark(old.Device, errCode)
					degrade(&old, "identity-unavailable")
					results[i] = old
					return
				}
				defer conn.Close()
			}
			r := s.commandVia(c, conn, old.Target, "version")
			device, e := ParseDevice(r.Output)
			if r.Error == "" && device.MAC == "" {
				extra := s.commandVia(c, conn, old.Target, "identity-eeprom")
				if extra.Error == "" {
					device.MAC = ParseEEPROM(extra.Output)
					if device.MAC != "" {
						e = nil
						r.SampledAt = extra.SampledAt
						r.Command = "version+identity-eeprom"
					}
				}
			}
			old.Device = update(old.Device, r, device, e)
			if r.Error != "" || e != nil {
				old.IdentityEvidence = "Device MAC was not verified during this attempt: " + old.Device.Error
				// Preserve the last-verified UID when inventory still confirms this switch's MAC,
				// so the frontend can continue showing stale data during transient SSH failures.
				if previousUID != "" && previousUID == sw.Metadata.Key() {
					old.SwitchUID = previousUID
				}
				degrade(&old, "identity-unavailable")
				results[i] = old
				return
			}
			if device.MAC != mac(old.Target.ExpectedMAC) || (device.Hostname != "" && ((old.Target.ExpectedHostname != "" && device.Hostname != old.Target.ExpectedHostname) || (sw.Spec.Hostname != "" && device.Hostname != sw.Spec.Hostname))) {
				old.SwitchUID = ""
				old.IdentityState = "mismatch"
				old.IdentityEvidence = "Observed identity conflicts with explicit target or Kubernetes Switch declaration"
				degrade(&old, "identity-mismatch")
				results[i] = old
				return
			}
			old.IdentityState = "verified"
			old.SwitchUID = sw.Metadata.Key()
			old.IdentityEvidence = "Observed device base MAC matches explicit SSH mapping and current Switch declared MAC; host key verified by OpenSSH"
			r = s.commandVia(c, conn, old.Target, "ports")
			ports, e := ParsePorts(r.Output)
			old.Ports = update(old.Ports, r, ports, e)
			ipv4 := s.commandVia(c, conn, old.Target, "ipv4")
			a4, e4 := ParseAddresses(ipv4.Output, "ipv4")
			ipv6 := s.commandVia(c, conn, old.Target, "ipv6")
			a6, e6 := ParseAddresses(ipv6.Output, "ipv6")
			ar := ipv6
			ae := e6
			if ipv4.Error != "" {
				ar.Error = ipv4.Error
			}
			if e4 != nil && ae == nil {
				ae = e4
			}
			ar.Command = "ipv4+ipv6"
			old.Addresses = update(old.Addresses, ar, append(a4, a6...), ae)
			r = s.commandVia(c, conn, old.Target, "lldp-json")
			neighbors, e := ParseLLDP(r.Output)
			if r.Error == "unsupported-command" || e != nil && r.Error == "" {
				r = s.commandVia(c, conn, old.Target, "lldp-table")
				neighbors, e = ParseLLDPTable(r.Output)
			}
			old.LLDP = update(old.LLDP, r, neighbors, e)
			r = s.commandVia(c, conn, old.Target, "vlans")
			vlans, e := ParseVLANs(r.Output)
			old.VLANs = update(old.VLANs, r, vlans, e)
			r = s.commandVia(c, conn, old.Target, "docker-ps")
			containers, e := ParseContainers(r.Output)
			old.Containers = update(old.Containers, r, containers, e)
			r = s.commandVia(c, conn, old.Target, "bgp-summary-json")
			sessions, e := ParseBGPSummary(r.Output)
			if r.Error == "unsupported-command" || e != nil && r.Error == "" {
				r = s.commandVia(c, conn, old.Target, "bgp-ipv6-text")
				sessions, e = ParseBGPText(r.Output)
			}
			if r.Error == "" && e == nil {
				detail := s.commandVia(c, conn, old.Target, "bgp-neighbors-json")
				d, de := ParseBGP(detail.Output)
				old.BGPDetails = update(old.BGPDetails, detail, d, de)
				// Each command keeps its own timestamp; details cannot erase valid summary data.
			} else {
				old.BGPDetails = mark(old.BGPDetails, "summary-unavailable")
			}
			old.BGP = update(old.BGP, r, sessions, e)
			results[i] = old
		}(i, old)
	}
	wg.Wait()
	s.mu.Lock()
	s.switches = results
	s.mu.Unlock()
}
func degrade(sw *Switch, code string) {
	sw.Ports = mark(sw.Ports, code)
	sw.Addresses = mark(sw.Addresses, code)
	sw.BGP = mark(sw.BGP, code)
	sw.BGPDetails = mark(sw.BGPDetails, code)
	sw.LLDP = mark(sw.LLDP, code)
	sw.VLANs = mark(sw.VLANs, code)
	sw.Containers = mark(sw.Containers, code)
}
func age[T any](d Domain[T], now time.Time, limit time.Duration) Domain[T] {
	if d.SampledAt != nil && now.Sub(*d.SampledAt) > limit && (d.State == "ready" || d.State == "partial") {
		d.State = "stale"
		d.Error = "sample-expired"
	}
	return d
}
func (s *Store) Get() Snapshot {
	now := time.Now().UTC()
	s.mu.RLock()
	out := Snapshot{SchemaVersion: "v1", Mode: s.Mode, PollSeconds: s.Interval.Seconds(), StaleAfterSeconds: s.StaleAfter.Seconds(), GeneratedAt: now, Switches: append([]Switch{}, s.switches...), Links: []Link{}}
	s.mu.RUnlock()
	var inv model.Snapshot
	_ = json.Unmarshal(s.Inventory.Get().Body, &inv)
	valid := map[string]string{}
	validMAC := map[string]string{}
	fresh := false
	for _, st := range inv.Sources {
		if st.Kind == "switches" {
			fresh = st.State == "ready" && st.Complete
		}
	}
	for _, sw := range inv.Switches {
		valid[sw.Resource.Metadata.Name] = sw.Resource.Metadata.Key()
		validMAC[sw.Resource.Metadata.Name] = mac(sw.Resource.Spec.MACAddress)
	}
	for i := range out.Switches {
		sw := &out.Switches[i]
		sw.Device = age(sw.Device, now, s.StaleAfter)
		sw.Ports = age(sw.Ports, now, s.StaleAfter)
		sw.Addresses = age(sw.Addresses, now, s.StaleAfter)
		sw.BGP = age(sw.BGP, now, s.StaleAfter)
		sw.BGPDetails = age(sw.BGPDetails, now, s.StaleAfter)
		sw.LLDP = age(sw.LLDP, now, s.StaleAfter)
		sw.VLANs = age(sw.VLANs, now, s.StaleAfter)
		sw.Containers = age(sw.Containers, now, s.StaleAfter)
		if sw.SwitchUID != "" && (!fresh || valid[sw.Target.ResourceName] != sw.SwitchUID || validMAC[sw.Target.ResourceName] != mac(sw.Target.ExpectedMAC)) {
			sw.SwitchUID = ""
			sw.IdentityState = "inventory-unavailable"
			degrade(sw, "inventory-unavailable")
		}
		if sw.Device.State != "ready" && sw.IdentityState == "verified" {
			sw.IdentityState = "stale"
		}
	}
	if s.Mode == "ssh-lab" {
		out.Links = ResolveLinks(out.Switches, now, s.StaleAfter)
	} // Capture replay never asserts current physical links.
	return out
}
func (s *Store) Run(ctx context.Context) {
	if s.Reader == nil {
		return
	}
	s.Refresh(ctx)
	timer := time.NewTicker(s.Interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.Refresh(ctx)
		}
	}
}

// ValidateContainerRequest checks that the given switchUID and containerID
// are present in the current snapshot. Returns the Target, the canonical
// container ID (to use in commands instead of the raw caller value), and true
// if valid; zero values and false otherwise.
func (s *Store) ValidateContainerRequest(switchUID, containerID string) (Target, string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sw := range s.switches {
		if sw.SwitchUID != switchUID {
			continue
		}
		for _, c := range sw.Containers.Data {
			if c.ID == containerID {
				return sw.Target, c.ID, true
			}
		}
		return Target{}, "", false // switch found but container not in its list
	}
	return Target{}, "", false
}
