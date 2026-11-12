import { Badge } from "./Inspector";
import { key, type Inventory, type SwitchView } from "./model";
import {
  domainState,
  type Domain,
  type ObservedSwitch,
  type Observations,
} from "./observations";
const unknown = "Not reported";
export function ObservationFleet({
  data,
  error,
}: {
  data?: Observations;
  error?: string;
}) {
  if (!data || data.mode === "disabled")
    return (
      <div className="note observation-status">
        SONiC observation collection {error ? "unavailable" : "disabled"}.
        Kubernetes resource refresh is independent.
      </div>
    );
  return (
    <section className="observation-fleet" aria-label="SONiC collection status">
      <div className="observation-heading">
        <strong>SONiC operational observations</strong>
        <Badge
          value={data.mode === "replay" ? "CAPTURE REPLAY" : "OBSERVATIONS"}
        />
        <span>
          {error || "Separate device samples · no configuration changes"}
        </span>
      </div>
      <div className="observation-targets">
        {data.switches.map((s) => (
          <div key={s.target.resourceName}>
            <strong>
              {s.target.expectedHostname || s.target.resourceName}
            </strong>
            <Badge value={s.identityState} />
            <small>
              {s.identityState === "verified"
                ? "Ports " +
                  domainState(s.ports, data.staleAfterSeconds) +
                  " · BGP " +
                  domainState(s.bgp, data.staleAfterSeconds) +
                  " · LLDP " +
                  domainState(s.lldp, data.staleAfterSeconds)
                : s.identityEvidence}
            </small>
          </div>
        ))}
      </div>
    </section>
  );
}
export function DomainNotice<T>({
  domain,
  limit = 90,
}: {
  domain: Domain<T>;
  limit?: number;
}) {
  const state = domainState(domain, limit);
  return (
    <div className="sample-notice">
      <Badge value={state} />
      <span>
        {domain.sampledAt
          ? "CLI sample: " + new Date(domain.sampledAt).toLocaleString()
          : "No successful device sample"}
      </span>
      <small>
        {domain.error && "Collection error: " + domain.error + " · "}
        {domain.command || "Command not attempted"} ·{" "}
        {domain.complete
          ? "Complete command result"
          : "Incomplete / unavailable"}
      </small>
    </div>
  );
}
export function IdentityNotice({
  observation,
  mode,
}: {
  observation?: ObservedSwitch;
  mode?: string;
}) {
  return (
    <div className="note">
      <strong>
        SONiC {mode === "replay" ? "capture replay" : "observations"}
      </strong>
      <p>
        {observation?.identityEvidence ||
          "No independently verified SONiC observations for this Switch. Kubernetes Ready/Cell Active are not telemetry."}
      </p>
      {observation && <Badge value={observation.identityState} />}
    </div>
  );
}
export function ObservedPorts({
  observation,
  data,
  sw,
  limit,
}: {
  observation: ObservedSwitch;
  data: Inventory;
  sw: SwitchView;
  limit: number;
}) {
  const node = data.nodes.find(
    (n) => key(n.resource.metadata) === sw.association.nodeKey,
  );
  const interfaces = data.interfaces.filter((i) =>
    node?.interfaceKeys.includes(key(i.metadata)),
  );
  return (
    <>
      <h3>SONiC-observed interfaces</h3>
      <DomainNotice domain={observation.ports} limit={limit} />
      {(observation.ports.data || []).map((port) => {
        const registered = interfaces.filter(
          (i) => i.spec.handle === "sonic://" + port.name,
        );
        const i = registered.length === 1 ? registered[0] : undefined;
        return (
          <div className="observed-port" key={port.name}>
            <div className="port-heading">
              <strong>{port.name}</strong>
              <Badge value={port.oper} />
            </div>
            <div className="state-columns">
              <div>
                <small>Kubernetes desired admin</small>
                <strong>{i?.spec.adminState || "Unknown"}</strong>
              </div>
              <div>
                <small>Kubernetes API operation</small>
                <strong>{i?.status.operationState || "Unknown"}</strong>
              </div>
              <div>
                <small>SONiC admin / operation</small>
                <strong>
                  {port.admin} / {port.oper}
                </strong>
              </div>
            </div>
            <p>
              {port.speed || unknown} · MTU {port.mtu ?? unknown} · FEC{" "}
              {port.fec || unknown}
            </p>
            <small>
              Fabric Interface:{" "}
              {i?.metadata.name ||
                (registered.length > 1
                  ? "Ambiguous provider handle"
                  : "No unique local provider-handle match")}
            </small>
          </div>
        );
      })}
      {observation.ports.data?.length === 0 && observation.ports.complete && (
        <p className="empty-inline">Command returned no interfaces.</p>
      )}
      <h3>SONiC IP interfaces</h3>
      <DomainNotice domain={observation.addresses} limit={limit} />
      {(observation.addresses.data || []).map((a, i) => (
        <div className="prefix" key={i}>
          <strong>{a.name || "Interface unknown"}</strong>
          <code>{a.address}</code>
          <small>{a.family}</small>
        </div>
      ))}
    </>
  );
}
export function BGPPanel({
  observation,
  limit,
}: {
  observation: ObservedSwitch;
  limit: number;
}) {
  return (
    <>
      <h3>Observed BGP sessions</h3>
      <DomainNotice domain={observation.bgp} limit={limit} />
      {observation.bgpDetails && (
        <>
          <h4>Neighbor-detail enrichment</h4>
          <DomainNotice domain={observation.bgpDetails} limit={limit} />
        </>
      )}
      <p className="note">
        FRR-reported sessions, including interface-named IPv6 unnumbered
        neighbors. Cell Active does not prove convergence. Missing counters or
        local interface identities remain unreported.
      </p>
      <div className="telemetry-table">
        <table>
          <thead>
            <tr>
              <th>Neighbor / VRF / family</th>
              <th>State / uptime</th>
              <th>ASN / prefixes</th>
            </tr>
          </thead>
          <tbody>
            {(observation.bgp.data || []).map((summary, i) => {
              const detail =
                observation.bgpDetails &&
                domainState(observation.bgpDetails, limit) === "ready"
                  ? observation.bgpDetails.data?.find(
                      (d) =>
                        d.neighbor === summary.neighbor &&
                        d.vrf === summary.vrf &&
                        d.addressFamily === summary.addressFamily,
                    )
                  : undefined;
              const b = {
                ...detail,
                ...summary,
                advertisedPrefixes:
                  detail?.advertisedPrefixes ?? summary.advertisedPrefixes,
              };
              return (
                <tr key={i}>
                  <td>
                    <strong>{b.neighbor}</strong>
                    <small>
                      {b.vrf} · {b.addressFamily}
                    </small>
                    <small>Local: {b.localInterface || unknown}</small>
                  </td>
                  <td>
                    <Badge value={b.state} />
                    <small>{b.uptime || unknown}</small>
                  </td>
                  <td>
                    AS {b.remoteASN ?? unknown}
                    <small>
                      Received {b.receivedPrefixes ?? unknown} · advertised{" "}
                      {b.advertisedPrefixes ?? unknown}
                    </small>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      {observation.bgp.complete && observation.bgp.data?.length === 0 && (
        <p className="empty-inline">
          No sessions returned by this command. This is not a health verdict.
        </p>
      )}
    </>
  );
}
export function LLDPPanel({
  observation,
  limit,
}: {
  observation: ObservedSwitch;
  limit: number;
}) {
  return (
    <>
      <h3>Observed LLDP neighbors</h3>
      <DomainNotice domain={observation.lldp} limit={limit} />
      <p className="note">
        Edges require verified device MACs and native ports. Reciprocal evidence
        is preferred; one-sided evidence is dashed. Reported daemon age is not a
        reliable last-received timestamp.
      </p>
      {observation.lldp.command === "lldp-table" && (
        <p className="note">
          Table output omits chassis/port subtypes. These neighbors cannot
          establish physical graph edges.
        </p>
      )}
      {(observation.lldp.data || []).map((n, i) => (
        <div className="observed-port" key={i}>
          <h3>
            {n.localPort} → {n.systemName || "System name unknown"}
          </h3>
          <p>
            Chassis: {n.chassisID || unknown} (
            {n.chassisSubtype || "Subtype unknown"})
          </p>
          <p>
            Remote port: {n.portID} ({n.portSubtype || "Subtype unknown"})
          </p>
          <small>
            Reported age: {n.reportedAge || unknown} · advertised TTL:{" "}
            {n.advertisedTTLSeconds ?? unknown}s
          </small>
        </div>
      ))}
    </>
  );
}
export function VLANPanel({
  observation,
  limit,
}: {
  observation: ObservedSwitch;
  limit: number;
}) {
  return (
    <>
      <h3>SONiC-reported VLAN configuration</h3>
      <DomainNotice domain={observation.vlans} limit={limit} />
      <p className="note">
        show vlan brief reports configured membership and tagging. Operational
        forwarding state was not measured.
      </p>
      <div className="telemetry-table">
        <table>
          <thead>
            <tr>
              <th>VLAN</th>
              <th>Member</th>
              <th>Tagging</th>
            </tr>
          </thead>
          <tbody>
            {(observation.vlans.data || []).map((v, i) => (
              <tr key={i}>
                <td>{v.id}</td>
                <td>{v.member || unknown}</td>
                <td>{v.tagging || unknown}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {observation.vlans.complete && observation.vlans.data?.length === 0 && (
        <p className="empty-inline">No VLAN entries returned by the command.</p>
      )}
    </>
  );
}
