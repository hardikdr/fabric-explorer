import { Badge } from "./Inspector";
import { key, type Inventory, type SwitchView } from "./model";
import {
  domainState,
  type Domain,
  type ObservedSwitch,
  type ObservedContainer,
  type Observations,
} from "./observations";
import { useState } from "react";
const unknown = "—";

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
        SONiC observations {error ? "unavailable" : "disabled"}.
      </div>
    );
  return (
    <section className="observation-fleet" aria-label="SONiC observation status">
      <div className="observation-heading">
        <strong>SONiC observations</strong>
        <Badge value={data.mode === "replay" ? "REPLAY" : "LIVE"} />
        <span>
          {new Date(data.generatedAt).toLocaleTimeString()} · every{" "}
          {data.pollSeconds}s
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
          ? new Date(domain.sampledAt).toLocaleTimeString()
          : "No sample"}
      </span>
      {domain.error && <small className="sample-error">{domain.error}</small>}
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
  if (!observation) {
    return (
      <div className="identity-notice identity-notice--absent">
        No SSH observations for this switch.
        {mode === "replay" ? " (capture replay)" : ""}
      </div>
    );
  }
  return (
    <div
      className={
        "identity-notice identity-notice--" +
        (observation.identityState === "verified" ? "ok" : "warn")
      }
    >
      <Badge value={observation.identityState} />
      <span>{observation.identityEvidence}</span>
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

  const allPorts = observation.ports.data || [];
  const fabricHandles = new Set(interfaces.map((i) => i.spec.handle));
  const fabricPorts = allPorts.filter((p) =>
    fabricHandles.has("sonic://" + p.name),
  );
  const otherPorts = allPorts.filter(
    (p) => !fabricHandles.has("sonic://" + p.name),
  );

  const PortRow = ({
    port,
    showFabric,
  }: {
    port: (typeof allPorts)[number];
    showFabric: boolean;
  }) => {
    const registered = interfaces.filter(
      (i) => i.spec.handle === "sonic://" + port.name,
    );
    const iface = registered.length === 1 ? registered[0] : undefined;
    const isDown =
      port.oper?.toLowerCase() === "down" && port.admin?.toLowerCase() === "up";
    return (
      <div
        className={
          "observed-port" + (isDown ? " observed-port--down" : "")
        }
        key={port.name}
      >
        <div className="port-heading">
          <strong>{port.name}</strong>
          {port.alias && (
            <span className="port-alias">{port.alias}</span>
          )}
          <Badge value={port.oper} />
        </div>
        <div className="port-meta">
          <span>{port.speed || unknown}</span>
          <span>MTU {port.mtu ?? unknown}</span>
          {showFabric && iface && (
            <span className="port-fabric-ref">{iface.metadata.name}</span>
          )}
        </div>
      </div>
    );
  };

  return (
    <>
      <DomainNotice domain={observation.ports} limit={limit} />
      {fabricPorts.length > 0 && (
        <>
          <h3 className="ports-section-title">
            Fabric ports{" "}
            <span className="count">{fabricPorts.length}</span>
          </h3>
          {fabricPorts.map((port) => (
            <PortRow port={port} showFabric key={port.name} />
          ))}
        </>
      )}
      {otherPorts.length > 0 && (
        <details className="ports-other">
          <summary>
            All other ports{" "}
            <span className="count">{otherPorts.length}</span>
          </summary>
          {otherPorts.map((port) => (
            <PortRow port={port} showFabric={false} key={port.name} />
          ))}
        </details>
      )}
      {allPorts.length === 0 && observation.ports.complete && (
        <p className="empty-inline">No interfaces returned.</p>
      )}
      {(observation.addresses.data || []).length > 0 && (
        <>
          <h3 className="ports-section-title">IP interfaces</h3>
          <DomainNotice domain={observation.addresses} limit={limit} />
          {observation.addresses.data!.map((a, i) => (
            <div className="prefix" key={i}>
              <strong>{a.name || "—"}</strong>
              <code>{a.address}</code>
              <small>{a.family}</small>
            </div>
          ))}
        </>
      )}
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
      <DomainNotice domain={observation.bgp} limit={limit} />
      <div className="telemetry-table">
        <table>
          <thead>
            <tr>
              <th>Neighbor</th>
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
                      {b.vrf !== "default" ? b.vrf + " · " : ""}
                      {b.addressFamily}
                      {b.localInterface ? " · " + b.localInterface : ""}
                    </small>
                  </td>
                  <td>
                    <Badge value={b.state} />
                    <small>{b.uptime || unknown}</small>
                  </td>
                  <td>
                    AS {b.remoteASN ?? unknown}
                    <small>
                      ↓{b.receivedPrefixes ?? "?"} ↑
                      {b.advertisedPrefixes ?? "?"}
                    </small>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      {observation.bgp.complete && observation.bgp.data?.length === 0 && (
        <p className="empty-inline">No sessions returned.</p>
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
      <DomainNotice domain={observation.lldp} limit={limit} />
      {observation.lldp.command === "lldp-table" && (
        <p className="note">
          Table output omits chassis/port subtypes. Edges require detail output.
        </p>
      )}
      {(observation.lldp.data || []).map((n, i) => (
        <div className="observed-port" key={i}>
          <div className="port-heading">
            <strong>{n.localPort}</strong>
            <span className="port-arrow">→</span>
            <strong>{n.systemName || "?"}</strong>
          </div>
          <div className="port-meta">
            <span>{n.portID}</span>
            {n.chassisID && <span>{n.chassisID}</span>}
            {n.advertisedTTLSeconds != null && (
              <span>TTL {n.advertisedTTLSeconds}s</span>
            )}
          </div>
        </div>
      ))}
      {(observation.lldp.data?.length ?? 0) === 0 &&
        observation.lldp.complete && (
          <p className="empty-inline">No LLDP neighbors observed.</p>
        )}
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
      <DomainNotice domain={observation.vlans} limit={limit} />
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
        <p className="empty-inline">No VLAN entries returned.</p>
      )}
    </>
  );
}

function ContainerRow({
  container,
  switchUID,
  onShowLogs,
}: {
  container: ObservedContainer;
  switchUID?: string;
  onShowLogs: (c: ObservedContainer) => void;
}) {
  const isRunning = container.state?.toLowerCase() === "running";
  return (
    <tr className={"container-row" + (isRunning ? "" : " container-row--stopped")}>
      <td className="container-name">{container.name}</td>
      <td>
        <Badge value={container.state} />
      </td>
      <td className="container-image" title={container.image}>
        {container.image.replace(/^.*\//, "")}
      </td>
      <td className="container-uptime">{container.runningFor || "—"}</td>
      <td>
        {switchUID && (
          <button
            className="inline-action container-logs-btn"
            onClick={() => onShowLogs(container)}
            aria-label={"Fetch logs for " + container.name}
          >
            Logs
          </button>
        )}
      </td>
    </tr>
  );
}

export function ContainersPanel({
  observation,
  limit,
}: {
  observation: ObservedSwitch;
  limit: number;
}) {
  const [selectedContainer, setSelectedContainer] =
    useState<ObservedContainer | null>(null);
  const [logs, setLogs] = useState<string>("");
  const [logError, setLogError] = useState<string>("");
  const [logLoading, setLogLoading] = useState(false);

  const state = domainState(observation.containers, limit);
  const containers = observation.containers?.data ?? [];
  const switchUID = observation.switchUID;

  async function fetchLogs(container: ObservedContainer) {
    if (!switchUID) return;
    setSelectedContainer(container);
    setLogs("");
    setLogError("");
    setLogLoading(true);
    try {
      const url =
        "/api/v1/containers/logs?switchUID=" +
        encodeURIComponent(switchUID) +
        "&containerID=" +
        encodeURIComponent(container.id);
      const res = await fetch(url, { signal: AbortSignal.timeout(35000) });
      const text = await res.text();
      if (!res.ok) {
        setLogError(res.headers.get("X-Log-Error") || "fetch-failed");
      } else {
        setLogs(text);
        if (res.headers.get("X-Log-Error")) {
          setLogError(res.headers.get("X-Log-Error")!);
        }
      }
    } catch {
      setLogError("request-failed");
    } finally {
      setLogLoading(false);
    }
  }

  return (
    <>
      <div className="domain-notice-row">
        <DomainNotice domain={observation.containers} limit={limit} />
      </div>
      {state === "unavailable" && (
        <p className="empty-inline">Container data not available.</p>
      )}
      {containers.length > 0 && (
        <div className="table-scroll">
          <table className="data-table containers-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>State</th>
                <th>Image</th>
                <th>Uptime</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {containers.map((c) => (
                <ContainerRow
                  key={c.id}
                  container={c}
                  switchUID={switchUID}
                  onShowLogs={fetchLogs}
                />
              ))}
            </tbody>
          </table>
        </div>
      )}
      {containers.length === 0 && state === "ready" && (
        <p className="empty-inline">No containers found.</p>
      )}
      {selectedContainer && (
        <div className="log-viewer">
          <div className="log-viewer-header">
            <strong>{selectedContainer.name}</strong>
            <span className="log-viewer-meta">
              {logLoading ? "Loading…" : "Last 30 lines"}
            </span>
            <button
              className="icon-button"
              onClick={() => {
                setSelectedContainer(null);
                setLogs("");
                setLogError("");
              }}
              aria-label="Close log viewer"
            >
              ✕
            </button>
          </div>
          {logError && (
            <div className="log-error">
              Log fetch: <code>{logError}</code>
            </div>
          )}
          {logs && (
            <pre className="log-output" aria-label="Container logs">
              {logs}
            </pre>
          )}
          {logLoading && <div className="log-loading">Fetching…</div>}
        </div>
      )}
    </>
  );
}
