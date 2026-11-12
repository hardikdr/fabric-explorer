import { useMemo, useState } from "react";
import {
  Network,
  Search,
  LockKeyhole,
  LayoutDashboard,
  Boxes,
  Radio,
  ArrowUpRight,
  RefreshCw,
  SlidersHorizontal,
  PanelRightOpen,
  Table2,
  GitBranch,
  AlertTriangle,
  ChevronDown,
} from "lucide-react";
import { useInventory } from "./api";
import { key, filterSwitches, type Inventory, type Filters } from "./model";
import {
  Badge,
  Inspector,
  CellDetail,
  Conditions,
  Diagnostics,
} from "./Inspector";
import { Topology } from "./Topology";
import { ThemeSelector } from "./theme";
import {
  eligibleLinks,
  useObservations,
  type Observations,
} from "./observations";
import { ObservationFleet } from "./ObservedPanels";
export function App() {
  const { data, error, loading } = useInventory();
  const observations = useObservations();
  if (!data)
    return (
      <div className="startup">
        <Network size={38} />
        <h1>Fabric Explorer</h1>
        <p>
          {loading
            ? "Reading fabric inventory…"
            : error || "Inventory unavailable"}
        </p>
        {error && <button onClick={() => location.reload()}>Retry</button>}
      </div>
    );
  return (
    <Dashboard
      data={data}
      error={error}
      observations={observations.data}
      observationError={observations.error}
    />
  );
}
function Sources({ data }: { data: Inventory }) {
  return (
    <details className="sources">
      <summary>
        <RefreshCw size={14} />
        Resource source details
        <ChevronDown size={14} />
      </summary>
      <div className="source-grid">
        {data.sourceStates.map((s) => (
          <div className="source-card" key={s.kind}>
            <strong>
              {s.kind} <Badge value={s.state} />
            </strong>
            <p>
              {s.count} retained objects ·{" "}
              {s.complete ? "complete list" : "incomplete / uncertain"}
            </p>
            <p>Scope: {s.scope.join(", ")}</p>
            <small>
              Last successful list:{" "}
              {s.lastSuccessfulListAt
                ? new Date(s.lastSuccessfulListAt).toLocaleTimeString()
                : "never"}
              <br />
              Last attempt:{" "}
              {s.lastAttemptAt
                ? new Date(s.lastAttemptAt).toLocaleTimeString()
                : "not yet"}
              {s.error && (
                <>
                  <br />
                  Error: {s.error}
                </>
              )}
            </small>
          </div>
        ))}
      </div>
      <p className="note">
        Kubernetes lists are not transactional. Refresh time does not establish
        device status freshness. Partial lists retain previous objects until a
        complete refresh.
      </p>
    </details>
  );
}
function Unmatched({ data }: { data: Inventory }) {
  return (
    <details className="unmatched">
      <summary>
        <Boxes size={16} />
        Unmatched Fabric resources{" "}
        <span className="count">
          {Object.values(data.unmatched).reduce((sum, v) => sum + v.length, 0)}
        </span>
        <ChevronDown size={14} />
      </summary>
      <p className="note">
        These resources have no resolved Switch association. Names and
        references are retained for inspection.
      </p>
      {data.nodes
        .filter((n) => data.unmatched.nodes.includes(key(n.resource.metadata)))
        .map((n) => (
          <details key={key(n.resource.metadata)}>
            <summary>
              Node · {n.resource.metadata.name} <Badge value={n.binding} />
            </summary>
            <pre>{JSON.stringify(n.resource, null, 2)}</pre>
            <Diagnostics items={n.diagnostics} />
          </details>
        ))}
      {data.cells
        .filter((c) => data.unmatched.cells.includes(key(c.resource.metadata)))
        .map((c) => (
          <details key={key(c.resource.metadata)}>
            <summary>
              Cell · {c.resource.metadata.namespace}/{c.resource.metadata.name}
            </summary>
            <CellDetail cell={c} data={data} />
          </details>
        ))}
      {data.interfaces
        .filter((i) => data.unmatched.interfaces.includes(key(i.metadata)))
        .map((i) => (
          <details key={key(i.metadata)}>
            <summary>
              Interface · {i.metadata.name}{" "}
              <Badge value={i.status.operationState} />
            </summary>
            <pre>{JSON.stringify(i, null, 2)}</pre>
            <Conditions identity={i.metadata} status={i.status} />
          </details>
        ))}
      {Object.values(data.unmatched).every((v) => v.length === 0) && (
        <p className="empty-inline">
          All listed Fabric resources have a resolved Switch association.
        </p>
      )}
    </details>
  );
}
export function Dashboard({
  data,
  error = "",
  observations,
  observationError = "",
}: {
  data: Inventory;
  error?: string;
  observations?: Observations;
  observationError?: string;
}) {
  const [selected, setSelected] = useState(
    () => new URLSearchParams(location.search).get("switch") || "",
  );
  const [filters, setFilters] = useState<Filters>({
    search: "",
    role: "",
    association: "",
    namespace: "",
    phase: "",
  });
  const [view, setView] = useState("both");
  const [physical, setPhysical] = useState(false);
  const [showFilters, setShowFilters] = useState(false);
  const switches = useMemo(
    () => filterSwitches(data, filters),
    [data, filters],
  );
  const sw = data.switches.find((s) => key(s.resource.metadata) === selected);
  const select = (id: string) => {
    setSelected(id);
    const url = new URL(location.href);
    if (id) url.searchParams.set("switch", id);
    else url.searchParams.delete("switch");
    history.replaceState(null, "", url);
  };
  const mapped = data.switches.filter(
    (s) => s.association.state === "associated",
  ).length;
  const degraded = data.sourceStates.some(
    (s) => s.state !== "ready" || !s.complete,
  );
  const chooseFilter = (field: keyof Filters, value: string) =>
    setFilters((f) => ({ ...f, [field]: value }));
  return (
    <div className="app">
      <nav className="sidebar" aria-label="Main navigation">
        <div className="brand-symbol">
          <Network size={26} />
        </div>
        <span className="brand-name">IRONCORE</span>
        <div className="nav-item selected" title="Fabric Explorer">
          <LayoutDashboard size={21} />
          <span>Explorer</span>
        </div>
        <div className="nav-divider" />
        <div className="sidebar-bottom">
          <LockKeyhole size={19} />
          <span>Read only</span>
          <small>v0.2.0</small>
        </div>
      </nav>
      <div className="shell">
        <header className="header">
          <div className="breadcrumb">
            Infrastructure <span>/</span> Fabric Explorer
          </div>
          <div className="header-right">
            <ThemeSelector />
            <span className={"mode " + data.mode}>
              <span />
              {data.mode === "fixture"
                ? "FIXTURE DATA"
                : "KUBERNETES INVENTORY"}
            </span>
            <span className="readonly">
              <LockKeyhole size={13} />
              Read only
            </span>
          </div>
        </header>
        <main>
          <div className="page-heading">
            <div>
              <div className="eyebrow">NETWORK INFRASTRUCTURE</div>
              <h1>Fabric Explorer</h1>
              <p>
                Understand your switches, configuration, and resource
                relationships.
              </p>
            </div>
            <div className="sync">
              <span
                className={"sync-dot " + (degraded || error ? "degraded" : "")}
              />
              <div>
                <strong>
                  {error
                    ? "UI refresh failed"
                    : degraded
                      ? "Resource sources degraded"
                      : "Kubernetes resources refreshed"}
                </strong>
                <small>
                  {data.mode === "fixture" ? "Fixture replay · " : ""}
                  {new Date(data.generatedAt).toLocaleTimeString()} · every{" "}
                  {data.pollSeconds}s
                </small>
              </div>
            </div>
          </div>
          <div className="context">
            <Boxes size={13} />
            <span>{data.context}</span>
            <span className="separator">·</span>
            <span>
              Kubernetes configuration and API state. Device observations are
              not provided by this build.
            </span>
          </div>
          {(degraded || error) && (
            <div className="warning-banner" role="status">
              <AlertTriangle size={19} />
              <div>
                <strong>{error || "Inventory is incomplete or stale"}</strong>
                <p>
                  Retained resources may be outdated. Missing references cannot
                  be confirmed while their source is unavailable.
                </p>
              </div>
            </div>
          )}
          <section className="summary" aria-label="Fleet summary">
            {[
              {
                label: "Declared switches",
                value: data.switches.length,
                sub: "SONiC Switch resources",
                icon: Network,
              },
              {
                label: "Associated Nodes",
                value: mapped,
                sub: data.switches.length - mapped + " switches unresolved",
                icon: Boxes,
              },
              {
                label: "Bound Cells",
                value: data.cells.filter((c) => c.binding === "bound").length,
                sub:
                  data.cells.filter((c) => c.binding !== "bound").length +
                  " other targeted / orphan Cells",
                icon: GitBranch,
              },
              {
                label: "Reference diagnostics",
                value: data.diagnostics.length,
                sub: "Configuration evidence, not health",
                icon: AlertTriangle,
              },
            ].map(({ label, value, sub, icon: Icon }) => (
              <div className="stat-card" key={label}>
                <div>
                  <span>{label}</span>
                  <Icon size={18} />
                </div>
                <strong>{value}</strong>
                <small>{sub}</small>
              </div>
            ))}
          </section>
          <Sources data={data} />
          <ObservationFleet data={observations} error={observationError} />
          <div className={"workspace " + (sw ? "inspecting" : "")}>
            <div className="explorer">
              <div className="toolbar">
                <div className="search">
                  <Search size={17} />
                  <input
                    aria-label="Search switches"
                    placeholder="Search switches, hostnames, Nodes, IPs…"
                    value={filters.search}
                    onChange={(e) => chooseFilter("search", e.target.value)}
                  />
                  {filters.search && (
                    <button
                      onClick={() => chooseFilter("search", "")}
                      aria-label="Clear search"
                    >
                      ×
                    </button>
                  )}
                </div>
                <select
                  aria-label="Filter by role"
                  value={filters.role}
                  onChange={(e) => chooseFilter("role", e.target.value)}
                >
                  <option value="">All roles</option>
                  <option value="spine">Spines</option>
                  <option value="leaf">Leaves</option>
                  <option value="unknown">Unclassified</option>
                </select>
                <button
                  className={"filter-button " + (showFilters ? "active" : "")}
                  onClick={() => setShowFilters((v) => !v)}
                >
                  <SlidersHorizontal size={16} />
                  <span>Filters</span>
                </button>
                <div className="view-toggle" aria-label="View">
                  <button
                    className={view !== "table" ? "active" : ""}
                    aria-label="Show logical topology and inventory"
                    onClick={() => setView("both")}
                  >
                    <GitBranch size={17} />
                  </button>
                  <button
                    className={view === "table" ? "active" : ""}
                    aria-label="Show inventory only"
                    onClick={() => setView("table")}
                  >
                    <Table2 size={17} />
                  </button>
                </div>
              </div>
              {showFilters && (
                <div className="filters">
                  <select
                    aria-label="Filter by association"
                    value={filters.association}
                    onChange={(e) =>
                      chooseFilter("association", e.target.value)
                    }
                  >
                    <option value="">All associations</option>
                    {[
                      "associated",
                      "unassociated",
                      "ambiguous",
                      "conflicting",
                      "unresolved",
                    ].map((s) => (
                      <option key={s}>{s}</option>
                    ))}
                  </select>
                  <select
                    aria-label="Filter by namespace"
                    value={filters.namespace}
                    onChange={(e) => chooseFilter("namespace", e.target.value)}
                  >
                    <option value="">All Cell namespaces</option>
                    {[
                      ...new Set(
                        data.cells.map(
                          (c) => c.resource.metadata.namespace || "",
                        ),
                      ),
                    ]
                      .filter(Boolean)
                      .map((ns) => (
                        <option key={ns}>{ns}</option>
                      ))}
                  </select>
                  <select
                    aria-label="Filter by phase"
                    value={filters.phase}
                    onChange={(e) => chooseFilter("phase", e.target.value)}
                  >
                    <option value="">All Cell phases</option>
                    {["Pending", "Active", "Expired", "Failed"].map((p) => (
                      <option key={p}>{p}</option>
                    ))}
                  </select>
                  <button
                    onClick={() =>
                      setFilters({
                        search: "",
                        role: "",
                        association: "",
                        namespace: "",
                        phase: "",
                      })
                    }
                  >
                    Reset
                  </button>
                </div>
              )}
              {view !== "table" && (
                <section className="topology-card">
                  <div className="card-heading">
                    <div>
                      <h2>Logical configuration view</h2>
                      <p>
                        {physical
                          ? "LLDP-backed observations · missing/stale evidence suppresses edges"
                          : "Logical configuration grouping · no cabling inferred"}
                      </p>
                      <button
                        className="topology-mode"
                        aria-pressed={physical}
                        onClick={() => setPhysical((v) => !v)}
                      >
                        {physical
                          ? "Show logical view"
                          : "Show observed LLDP links"}{" "}
                        (
                        {observationError
                          ? 0
                          : eligibleLinks(observations).length}
                        )
                      </button>
                    </div>
                    <div className="legend">
                      <span className="spine-dot" />
                      Spine
                      <span className="leaf-dot" />
                      Leaf
                      <span className="unknown-dot" />
                      Unclassified
                    </div>
                  </div>
                  <Topology
                    links={
                      physical && !observationError
                        ? eligibleLinks(observations)
                        : []
                    }
                    physical={physical}
                    switches={switches}
                    selected={selected}
                    onSelect={select}
                  />
                </section>
              )}
              <section className="inventory-card">
                <div className="card-heading">
                  <div>
                    <h2>
                      Switch inventory{" "}
                      <span className="count">{switches.length}</span>
                    </h2>
                    <p>
                      {switches.length} of {data.switches.length} declared
                      switches
                    </p>
                  </div>
                  <span className="table-hint">
                    <PanelRightOpen size={15} />
                    Click to inspect
                  </span>
                </div>
                <div className="table-scroll">
                  <table>
                    <thead>
                      <tr>
                        <th>Switch / management</th>
                        <th>Role</th>
                        <th>Provisioning declaration</th>
                        <th>Fabric association / Cell</th>
                        <th />
                      </tr>
                    </thead>
                    <tbody>
                      {switches.map((s) => {
                        const n = data.nodes.find(
                          (n) =>
                            key(n.resource.metadata) === s.association.nodeKey,
                        );
                        const c = data.cells.find(
                          (c) => key(c.resource.metadata) === n?.boundCellKey,
                        );
                        return (
                          <tr
                            key={key(s.resource.metadata)}
                            className={
                              selected === key(s.resource.metadata)
                                ? "selected"
                                : ""
                            }
                          >
                            <td>
                              <button
                                className="switch-link"
                                onClick={() => select(key(s.resource.metadata))}
                              >
                                <span className={"table-device " + s.role}>
                                  <Network size={17} />
                                </span>
                                <span>
                                  <strong>{s.displayName}</strong>
                                  <small>
                                    {s.resource.spec.management.host ||
                                      "No management host"}{" "}
                                    · {s.resource.metadata.name}
                                  </small>
                                </span>
                              </button>
                            </td>
                            <td>
                              <Badge value={s.role} />
                            </td>
                            <td>
                              <Badge value={s.resource.status.state} />
                            </td>
                            <td>
                              <div className="association-cell">
                                <strong>
                                  {n?.resource.metadata.name ||
                                    s.association.state}
                                </strong>
                                <small>
                                  {c
                                    ? c.resource.metadata.namespace +
                                      "/" +
                                      c.resource.metadata.name
                                    : n
                                      ? "Cell binding: " + n.binding
                                      : "No resolved Node"}
                                </small>
                              </div>
                            </td>
                            <td>
                              <button
                                className="icon-button"
                                aria-label={"Inspect " + s.displayName}
                                onClick={() => select(key(s.resource.metadata))}
                              >
                                <ArrowUpRight size={16} />
                              </button>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                  {switches.length === 0 && (
                    <div className="empty-panel">
                      <Search size={28} />
                      <h3>
                        {data.switches.length
                          ? "No matching switches"
                          : "No Switch resources listed"}
                      </h3>
                      <p>
                        {data.switches.length
                          ? "Adjust the search or filters."
                          : "Check source status and scope. A complete empty list is distinct from a failed read."}
                      </p>
                    </div>
                  )}
                </div>
                <div className="table-footer">
                  <Radio size={13} />
                  Ready is provisioning acceptance. API status is not a live
                  device health signal.
                </div>
              </section>
              <Unmatched data={data} />
              {data.diagnostics.length > 0 && (
                <details className="fleet-diagnostics">
                  <summary>
                    <AlertTriangle size={16} />
                    All reference diagnostics{" "}
                    <span className="count">{data.diagnostics.length}</span>
                  </summary>
                  <Diagnostics items={data.diagnostics} />
                </details>
              )}
            </div>
            {sw && (
              <Inspector
                sw={sw}
                data={data}
                observation={observations?.switches.find(
                  (o) => o.target.resourceName === sw.resource.metadata.name,
                )}
                observationMode={observations?.mode}
                observationLimit={observations?.staleAfterSeconds}
                observationError={observationError}
                onClose={() => select("")}
              />
            )}
          </div>
          {selected && !sw && (
            <div className="note">
              The selected resource is no longer in this snapshot.{" "}
              <button onClick={() => select("")}>Clear selection</button>
            </div>
          )}
          <footer>
            IronCore Fabric Explorer{" "}
            <span>
              Read-only · Existing Kubernetes APIs · No device control
            </span>
          </footer>
        </main>
      </div>
    </div>
  );
}
