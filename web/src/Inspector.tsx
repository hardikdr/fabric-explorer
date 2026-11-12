import {
  IdentityNotice,
  ObservedPorts,
  BGPPanel,
  LLDPPanel,
  VLANPanel,
} from "./ObservedPanels";
import type { ObservedSwitch } from "./observations";
import { useEffect, useState } from "react";
import {
  X,
  Network,
  ChevronRight,
  AlertTriangle,
  FileText,
} from "lucide-react";
import {
  key,
  type Identity,
  type Status,
  type Inventory,
  type SwitchView,
  type CellView,
  type Diagnostic,
  type Resource,
  type InterfaceSpec,
} from "./model";
export function Badge({ value }: { value?: string }) {
  return (
    <span className={"badge " + (value || "unknown").toLowerCase()}>
      {value || "Unknown"}
    </span>
  );
}
function Fields({ items }: { items: [string, React.ReactNode][] }) {
  return (
    <dl className="fields">
      {items.map(([label, value]) => (
        <div key={label}>
          <dt>{label}</dt>
          <dd>{value || <span className="muted">Not reported</span>}</dd>
        </div>
      ))}
    </dl>
  );
}
export function Diagnostics({ items }: { items: Diagnostic[] }) {
  return (
    <div className="diagnostics">
      {items.map((d, i) => (
        <div className="diagnostic" key={d.code + i}>
          <AlertTriangle size={15} />
          <div>
            <strong>{d.code.replaceAll("-", " ")}</strong>
            <p>{d.summary}</p>
          </div>
        </div>
      ))}
    </div>
  );
}
function IdentityFields({ i }: { i: Identity }) {
  return (
    <details className="resource-identity">
      <summary>Resource identity · {i.name}</summary>
      <Fields
        items={[
          ["Resource", (i.namespace ? i.namespace + "/" : "") + i.name],
          ["UID", i.uid],
          ["Generation / version", i.generation + " / " + i.resourceVersion],
          ...(i.deletionTimestamp
            ? [["Deleting since", i.deletionTimestamp] as [string, string]]
            : []),
        ]}
      />
    </details>
  );
}
export function Conditions({
  status,
  identity,
}: {
  status: Status;
  identity: Identity;
}) {
  const conditions = status.conditions || [];
  return (
    <div>
      {conditions.length === 0 ? (
        <p className="empty-inline">No conditions reported by this resource.</p>
      ) : (
        conditions.map((c) => (
          <div className="condition" key={c.type}>
            <div>
              <strong>{c.type}</strong>
              <Badge value={c.status} />
            </div>
            <p>
              {c.reason} · {c.message}
            </p>
            <small>
              Transition: {c.lastTransitionTime || "unknown"} · generation{" "}
              {c.observedGeneration ?? "not reported"}
              {c.observedGeneration != null &&
              c.observedGeneration < identity.generation
                ? " · outdated"
                : ""}
            </small>
          </div>
        ))
      )}
    </div>
  );
}
function Port({
  peer,
  data,
}: {
  peer: CellView["peers"][number];
  data: Inventory;
}) {
  const inface = data.interfaces.find(
    (i) => key(i.metadata) === peer.interfaceKey,
  );
  return (
    <div className="port">
      <div className="port-heading">
        <span className={"port-symbol " + peer.purpose} />
        <strong>
          {inface?.spec.handle.replace("sonic://", "") ||
            peer.interfaceRef.name}
        </strong>
        <Badge value={peer.resolution} />
      </div>
      <Fields
        items={[
          ["Interface ref", peer.interfaceRef.name],
          ["Desired admin", inface?.spec.adminState || "Unknown"],
          ["API-reported state", inface?.status.operationState || "Unknown"],
          ...(peer.prefix
            ? [["Ordered south prefix", peer.prefix] as [string, string]]
            : []),
          ...(peer.dhcpRelay
            ? [["DHCP relay", peer.dhcpRelay] as [string, string]]
            : []),
        ]}
      />
      <small>
        State observation time unknown. No remote endpoint supplied.
      </small>
      {inface && (
        <details>
          <summary>Interface identity and conditions</summary>
          <IdentityFields i={inface.metadata} />
          <Conditions identity={inface.metadata} status={inface.status} />
        </details>
      )}
    </div>
  );
}
export function CellDetail({
  cell,
  data,
}: {
  cell: CellView;
  data: Inventory;
}) {
  const c = cell.resource;
  return (
    <>
      <div className="section-heading">
        <h3>Cell configuration</h3>
        <Badge value={cell.binding} />
      </div>
      <IdentityFields i={c.metadata} />
      <Fields
        items={[
          ["Target Node", c.spec.nodeRef.name],
          ["Cell ID", c.spec.id],
          ["Hostname", c.spec.hostname],
          ["Role label", cell.role || "Absent"],
          ["API phase", <Badge value={c.status.phase} />],
          ["Loopback IPs", c.spec.ips?.join(", ")],
          ["Summary prefix [0]", c.spec.prefixes?.[0]],
        ]}
      />
      <p className="note">
        Active is an API runtime completion marker; it does not confirm
        convergence.
      </p>
      <details>
        <summary>All prefixes in source order</summary>
        {(c.spec.prefixes || []).map((p, i) => (
          <div className="prefix" key={i}>
            <span>[{i}]</span>
            <code>{p}</code>
          </div>
        ))}
      </details>
      <h3>
        Fabric-facing peers{" "}
        <span className="count">
          {cell.peers.filter((p) => p.purpose === "fabric").length}
        </span>
      </h3>
      {cell.peers
        .filter((p) => p.purpose === "fabric")
        .map((p, i) => (
          <Port peer={p} data={data} key={i} />
        ))}
      <h3>
        South-facing peers{" "}
        <span className="count">
          {cell.peers.filter((p) => p.purpose === "south").length}
        </span>
      </h3>
      {cell.peers
        .filter((p) => p.purpose === "south")
        .map((p, i) => (
          <Port peer={p} data={data} key={i} />
        ))}
      {cell.peers.length === 0 && (
        <p className="empty-inline">No peers declared.</p>
      )}
      <h3>Cell conditions</h3>
      <Conditions status={c.status} identity={c.metadata} />
      <Diagnostics items={cell.diagnostics} />
    </>
  );
}
function RegisteredPort({ inface }: { inface: Resource<InterfaceSpec> }) {
  return (
    <details className="registered-port">
      <summary>
        {inface.spec.handle || inface.metadata.name}{" "}
        <Badge value={inface.status.operationState} />
      </summary>
      <IdentityFields i={inface.metadata} />
      <Fields
        items={[
          ["Node ref", inface.spec.nodeRef.name],
          ["Desired admin", inface.spec.adminState],
          ["API-reported operation", inface.status.operationState || "Unknown"],
        ]}
      />
      <Conditions status={inface.status} identity={inface.metadata} />
    </details>
  );
}
export function Inspector({
  sw,
  data,
  onClose,
  observation,
  observationMode,
  observationLimit = 90,
  observationError,
}: {
  sw: SwitchView;
  data: Inventory;
  onClose: () => void;
  observation?: ObservedSwitch;
  observationMode?: string;
  observationLimit?: number;
  observationError?: string;
}) {
  const [tab, setTab] = useState("overview");
  const [cellKey, setCellKey] = useState("");
  const node = data.nodes.find(
    (n) => key(n.resource.metadata) === sw.association.nodeKey,
  );
  const cells = data.cells.filter((c) =>
    node?.cellKeys.includes(key(c.resource.metadata)),
  );
  useEffect(() => {
    setCellKey("");
    setTab("overview");
  }, [sw.resource.metadata.uid]);
  const cell = cells.find(
    (c) => key(c.resource.metadata) === (cellKey || node?.boundCellKey),
  );
  const interfaces = data.interfaces.filter((i) =>
    node?.interfaceKeys.includes(key(i.metadata)),
  );
  const trustedObservation =
    observation?.switchUID === key(sw.resource.metadata)
      ? observation
      : undefined;
  return (
    <aside className="inspector" aria-label="Switch inspector">
      <div className="inspector-title">
        <div className={"device-icon " + sw.role}>
          <Network size={23} />
        </div>
        <div>
          <small>SWITCH INSPECTOR</small>
          <h2>{sw.displayName}</h2>
          <span>{sw.resource.metadata.name}</span>
        </div>
        <button
          className="icon-button close"
          onClick={onClose}
          aria-label="Close inspector"
        >
          <X size={18} />
        </button>
      </div>
      <div className="inspector-tabs" role="tablist">
        {["overview", "cells", "ports", "bgp", "lldp", "vlans", "evidence"].map(
          (t) => (
            <button
              role="tab"
              aria-selected={tab === t}
              className={tab === t ? "active" : ""}
              onClick={() => setTab(t)}
              key={t}
            >
              {t === "cells"
                ? "Cell config"
                : t === "bgp"
                  ? "BGP Sessions"
                  : t === "lldp"
                    ? "LLDP"
                    : t}
            </button>
          ),
        )}
      </div>
      <div className="inspector-body">
        {(tab === "ports" ||
          tab === "bgp" ||
          tab === "lldp" ||
          tab === "vlans") && (
          <IdentityNotice observation={observation} mode={observationMode} />
        )}
        {observationError && (
          <div className="warning-banner">{observationError}</div>
        )}
        {tab === "bgp" && trustedObservation && (
          <BGPPanel observation={trustedObservation} limit={observationLimit} />
        )}
        {tab === "lldp" && trustedObservation && (
          <LLDPPanel
            observation={trustedObservation}
            limit={observationLimit}
          />
        )}
        {tab === "vlans" && trustedObservation && (
          <VLANPanel
            observation={trustedObservation}
            limit={observationLimit}
          />
        )}
        {tab === "overview" && (
          <>
            <div className="section-heading">
              <h3>Provisioning declaration</h3>
              <Badge value={sw.resource.status.state} />
            </div>
            <p className="note">
              Ready means accepted by the controller. Bootstrap completion and
              device reachability are not confirmed.
            </p>
            <IdentityFields i={sw.resource.metadata} />
            <Fields
              items={[
                [
                  "Declared hostname",
                  sw.resource.spec.hostname ||
                    sw.resource.metadata.name + " (default)",
                ],
                [
                  "Management",
                  [
                    sw.resource.spec.management.host,
                    sw.resource.spec.management.port,
                  ]
                    .filter(Boolean)
                    .join(":"),
                ],
                ["Declared MAC", sw.resource.spec.macAddress],
                ["Next boot mode", sw.resource.spec.nextBootMode || "None"],
                ["Association", <Badge value={sw.association.state} />],
                ["Bound role", <Badge value={sw.role} />],
              ]}
            />
            <h3>Fabric Node</h3>
            {node ? (
              <>
                <IdentityFields i={node.resource.metadata} />
                <Fields
                  items={[
                    ["Name", node.resource.metadata.name],
                    ["Provider ID", node.resource.spec.providerID],
                    ["Cell binding", <Badge value={node.binding} />],
                    [
                      "Cell ref",
                      node.resource.spec.cellRef
                        ? node.resource.spec.cellRef.namespace +
                          "/" +
                          node.resource.spec.cellRef.name
                        : "No cellRef",
                    ],
                    ["Referenced Cell UID", node.resource.spec.cellRef?.uid],
                  ]}
                />
                <button
                  className="inline-action"
                  onClick={() => setTab("cells")}
                >
                  Inspect targeted Cells <ChevronRight size={15} />
                </button>
              </>
            ) : (
              <p className="empty-inline">
                No verified Node association. Candidates:{" "}
                {sw.association.candidates.join(", ") || "none"}. See evidence.
              </p>
            )}
            <h3>Switch conditions</h3>
            <Conditions
              status={sw.resource.status}
              identity={sw.resource.metadata}
            />
            {node && (
              <>
                <h3>Node conditions</h3>
                <Conditions
                  status={node.resource.status}
                  identity={node.resource.metadata}
                />
              </>
            )}
            <Diagnostics items={sw.diagnostics} />
            {(sw.resource.status.firmwareVersion ||
              sw.resource.status.sku ||
              sw.resource.status.macAddress) && (
              <>
                <h3>API-reported device metadata</h3>
                <Fields
                  items={[
                    ["Firmware", sw.resource.status.firmwareVersion],
                    ["SKU", sw.resource.status.sku],
                    ["Reported MAC", sw.resource.status.macAddress],
                  ]}
                />
                <p className="note">
                  Observation age unknown; these fields may be left over from
                  the legacy agent.
                </p>
              </>
            )}
            <details>
              <summary>Resource labels</summary>
              {Object.entries(sw.resource.metadata.labels || {}).map(
                ([k, v]) => (
                  <div className="label-item" key={k}>
                    <code>{k}</code>
                    <span>{v}</span>
                  </div>
                ),
              )}
            </details>
          </>
        )}
        {tab === "cells" && (
          <>
            {node?.resource.spec.cellRef && (
              <div className="note">
                Node cellRef: {node.resource.spec.cellRef.namespace}/
                {node.resource.spec.cellRef.name} · {node.binding}. UID:{" "}
                {node.resource.spec.cellRef.uid}
              </div>
            )}
            <label className="field-label" htmlFor="cell-selector">
              Targeted Cells ({cells.length})
            </label>
            <select
              id="cell-selector"
              value={cell ? key(cell.resource.metadata) : ""}
              onChange={(e) => setCellKey(e.target.value)}
            >
              <option value="">Choose configuration to inspect</option>
              {cells.map((c) => (
                <option
                  key={key(c.resource.metadata)}
                  value={key(c.resource.metadata)}
                >
                  {c.resource.metadata.namespace}/{c.resource.metadata.name} ·{" "}
                  {c.binding} · {c.resource.status.phase || "Unknown"}
                </option>
              ))}
            </select>
            {cell ? (
              <CellDetail cell={cell} data={data} />
            ) : (
              <div className="empty-panel">
                <FileText size={30} />
                <h3>
                  {cells.length
                    ? "Choose a targeted Cell"
                    : "No accessible targeted Cells"}
                </h3>
                <p>
                  A candidate Cell is not automatically treated as bound
                  configuration.
                </p>
              </div>
            )}
            <Diagnostics items={node?.diagnostics || []} />
          </>
        )}
        {tab === "ports" && (
          <>
            {trustedObservation && (
              <ObservedPorts
                observation={trustedObservation}
                data={data}
                sw={sw}
                limit={observationLimit}
              />
            )}
            <h3>Declared Switch ports ({sw.declaredPorts.length})</h3>
            <p className="note">
              Port declarations are separate from Fabric registration. Matches
              require a unique local SONiC handle.
            </p>
            {sw.declaredPorts.map((p) => (
              <div className="declared-port" key={p.name}>
                <strong>{p.name}</strong>
                <Badge value={p.resolution} />
              </div>
            ))}
            {sw.declaredPorts.length === 0 && (
              <p className="empty-inline">No ports declared.</p>
            )}
            <h3>Local Fabric Interfaces ({interfaces.length})</h3>
            <p className="note">
              Desired admin and API-reported operation are separate. Observation
              time is unknown.
            </p>
            {interfaces.map((i) => (
              <RegisteredPort inface={i} key={key(i.metadata)} />
            ))}
            {interfaces.length === 0 && (
              <p className="empty-inline">No resolved local Interfaces.</p>
            )}
            <button className="inline-action" onClick={() => setTab("cells")}>
              View configured peer purpose and prefixes{" "}
              <ChevronRight size={15} />
            </button>
          </>
        )}
        {tab === "evidence" && (
          <>
            <h3>Association evidence</h3>
            <Badge value={sw.association.state} />
            {sw.association.evidence.map((e, i) => (
              <div className="evidence" key={i}>
                <small>{e.type}</small>
                <strong>{e.nodeName}</strong>
                <p>{e.detail}</p>
              </div>
            ))}
            {sw.association.evidence.length === 0 && (
              <p className="empty-inline">
                No recognized naming or bootstrap declaration evidence.
              </p>
            )}
            <h3>Diagnostics</h3>
            <Diagnostics items={sw.diagnostics} />
            {sw.diagnostics.length === 0 && (
              <p className="empty-inline">
                No association diagnostics. This is not a network health
                verdict.
              </p>
            )}
            <h3>Role provenance</h3>
            <p className="note">
              {sw.roleCellKey
                ? "Role comes from the label on UID-validated bound Cell " +
                  data.cells.find(
                    (c) => key(c.resource.metadata) === sw.roleCellKey,
                  )?.resource.metadata.name
                : "No role established from a valid bound Cell."}
            </p>
          </>
        )}
      </div>
    </aside>
  );
}
