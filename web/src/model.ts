export interface Identity {
  name: string;
  namespace?: string;
  uid: string;
  generation: number;
  resourceVersion: string;
  labels?: Record<string, string>;
  deletionTimestamp?: string;
}
export const key = (i: Identity) =>
  i.uid || [i.namespace || "", i.name].join("/");
export interface Condition {
  type: string;
  status: string;
  reason: string;
  message: string;
  lastTransitionTime: string;
  observedGeneration: number | null;
}
export interface Status {
  state?: string;
  phase?: string;
  operationState?: string;
  conditions: Condition[] | null;
  macAddress?: string;
  firmwareVersion?: string;
  sku?: string;
}
export interface Ref {
  name: string;
  namespace?: string;
  uid?: string;
}
export interface Resource<S> {
  apiVersion: string;
  kind: string;
  metadata: Identity;
  spec: S;
  status: Status;
}
export interface SwitchSpec {
  hostname: string;
  macAddress: string;
  management: { host: string; port: string };
  ports: { name: string }[] | null;
  nextBootMode?: string;
}
export interface NodeSpec {
  providerID: string;
  cellRef?: Ref;
}
export interface InterfaceSpec {
  nodeRef: Ref;
  handle: string;
  adminState: string;
}
export interface CellSpec {
  nodeRef: Ref;
  id: string;
  hostname: string;
  ips: string[] | null;
  prefixes: string[] | null;
  peers: Peer[] | null;
}
export interface Peer {
  interfaceRef: Ref;
  dhcpRelay?: string;
}
export interface Diagnostic {
  code: string;
  severity: string;
  summary: string;
  related: string[];
}
export interface Association {
  state: string;
  nodeKey?: string;
  candidates: string[];
  evidence: { type: string; nodeName: string; detail: string }[];
}
export interface PeerView extends Peer {
  purpose: string;
  prefix?: string;
  interfaceKey?: string;
  resolution: string;
}
export interface CellView {
  resource: Resource<CellSpec>;
  binding: string;
  role: string;
  peers: PeerView[];
  diagnostics: Diagnostic[];
}
export interface NodeView {
  resource: Resource<NodeSpec>;
  boundCellKey?: string;
  binding: string;
  cellKeys: string[];
  interfaceKeys: string[];
  diagnostics: Diagnostic[];
}
export interface SwitchView {
  resource: Resource<SwitchSpec>;
  displayName: string;
  association: Association;
  role: string;
  roleCellKey?: string;
  declaredPorts: { name: string; interfaceKey?: string; resolution: string }[];
  diagnostics: Diagnostic[];
}
export interface SourceState {
  kind: string;
  state: string;
  complete: boolean;
  count: number;
  lastAttemptAt: string;
  lastSuccessfulListAt?: string;
  resourceVersion?: string;
  error?: string;
  scope: string[];
}
export interface Inventory {
  schemaVersion: string;
  revision: number;
  generatedAt: string;
  mode: "fixture" | "live";
  context: string;
  pollSeconds: number;
  sourceStates: SourceState[];
  switches: SwitchView[];
  nodes: NodeView[];
  cells: CellView[];
  interfaces: Resource<InterfaceSpec>[];
  unmatched: Record<string, string[]>;
  diagnostics: Diagnostic[];
}
export interface Filters {
  search: string;
  role: string;
  association: string;
  namespace: string;
  phase: string;
}
export function filterSwitches(data: Inventory, f: Filters): SwitchView[] {
  return data.switches.filter((sw) => {
    const n = data.nodes.find(
      (n) => key(n.resource.metadata) === sw.association.nodeKey,
    );
    const cells = data.cells.filter((c) =>
      n?.cellKeys.includes(key(c.resource.metadata)),
    );
    const text = [
      sw.displayName,
      sw.resource.metadata.name,
      sw.resource.spec.management.host,
      sw.resource.spec.macAddress,
      ...sw.association.candidates,
      ...cells.map((c) => c.resource.metadata.name),
    ]
      .join(" ")
      .toLowerCase();
    return (
      text.includes(f.search.toLowerCase()) &&
      (!f.role || sw.role === f.role) &&
      (!f.association || sw.association.state === f.association) &&
      (!f.namespace ||
        cells.some((c) => c.resource.metadata.namespace === f.namespace)) &&
      (!f.phase || cells.some((c) => c.resource.status.phase === f.phase))
    );
  });
}
