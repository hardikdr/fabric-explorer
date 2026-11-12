import type { PhysicalLink } from "./observations";
import { useEffect, useMemo } from "react";
import {
  ReactFlow,
  useReactFlow,
  useStore,
  Background,
  Controls,
  BackgroundVariant,
  Handle,
  Position,
  type NodeProps,
  type Node,
} from "@xyflow/react";
import { Network, Server } from "lucide-react";
import "@xyflow/react/dist/style.css";
import { layout } from "./layout";
import { key, type SwitchView } from "./model";
// Refit on actual canvas/node-set changes, preserving manual pan/zoom otherwise.
function FitViewport({ contentKey }: { contentKey: string }) {
  const width = useStore((state) => state.width);
  const height = useStore((state) => state.height);
  const { fitView, viewportInitialized } = useReactFlow();
  useEffect(() => {
    if (!viewportInitialized || width <= 0 || height <= 0) return;
    const timer = setTimeout(() => {
      // fitView queues until node measurements are available, including with
      // controlled nodes whose public model deliberately omits DOM dimensions.
      void fitView({ padding: 0.12, maxZoom: 1.15, duration: 0 });
    }, 60);
    return () => clearTimeout(timer);
  }, [width, height, viewportInitialized, contentKey, fitView]);
  return null;
}
function SwitchNode({ data, selected }: NodeProps<Node<{ sw: SwitchView }>>) {
  const sw = data.sw;
  return (
    <div
      role="button"
      tabIndex={0}
      aria-label={"Inspect " + sw.displayName + " in topology"}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          event.currentTarget.click();
        }
      }}
      className={"switch-node " + sw.role + (selected ? " chosen" : "")}
    >
      <Handle type="target" position={Position.Top} style={{ opacity: 0 }} />
      <Handle type="source" position={Position.Bottom} style={{ opacity: 0 }} />
      <div className="node-top">
        <Network size={17} />
        <span>{sw.role === "unknown" ? "Unclassified" : sw.role}</span>
        <Server size={15} />
      </div>
      <strong>{sw.displayName}</strong>
      <div className="node-bottom">
        <span>{sw.resource.spec.management.host || "Management unknown"}</span>
        <span className="node-mark">
          {sw.association.state === "associated"
            ? "Associated"
            : "? " + sw.association.state}
        </span>
      </div>
    </div>
  );
}
function LabelNode({
  data,
}: NodeProps<Node<{ label: string; count: number; width: number }>>) {
  return (
    <div className="tier-label" style={{ width: data.width }}>
      {data.label}
      <span>{data.count} switches</span>
    </div>
  );
}
const nodeTypes = { switch: SwitchNode, label: LabelNode };
export function Topology({
  switches,
  selected,
  onSelect,
  links = [],
  physical = false,
}: {
  switches: SwitchView[];
  selected: string;
  onSelect: (id: string) => void;
  links?: PhysicalLink[];
  physical?: boolean;
}) {
  const nodes = useMemo(() => layout(switches, selected), [switches, selected]);
  return (
    <div className="flow">
      <ReactFlow
        nodes={nodes}
        edges={links
          .filter(
            (l) =>
              switches.some((s) => key(s.resource.metadata) === l.source) &&
              switches.some((s) => key(s.resource.metadata) === l.target),
          )
          .map((link) => {
            const sourceRole = switches.find(
              (s) => key(s.resource.metadata) === link.source,
            )?.role;
            const targetRole = switches.find(
              (s) => key(s.resource.metadata) === link.target,
            )?.role;
            const l =
              sourceRole === "leaf" && targetRole === "spine"
                ? {
                    ...link,
                    source: link.target,
                    target: link.source,
                    sourcePort: link.targetPort,
                    targetPort: link.sourcePort,
                  }
                : link;
            return {
              id: l.source + l.sourcePort + l.target + l.targetPort,
              source: l.source,
              target: l.target,
              label:
                selected === l.source || selected === l.target
                  ? l.sourcePort + " ↔ " + l.targetPort
                  : undefined,
              ariaLabel:
                l.sourcePort + " to " + l.targetPort + ": " + l.evidence,
              style: {
                stroke: "var(--teal)",
                strokeWidth: 2,
                strokeDasharray: l.evidence.startsWith("one-sided")
                  ? "6 4"
                  : undefined,
              },
              labelStyle: { fill: "var(--text)", fontSize: 10 },
              labelBgStyle: { fill: "var(--surface)" },
            };
          })}
        nodeTypes={nodeTypes}
        onNodeClick={(_, node) => {
          if (node.type === "switch") onSelect(node.id);
        }}
        nodesDraggable={false}
        nodesConnectable={false}
        connectOnClick={false}
        elementsSelectable
        fitView
        fitViewOptions={{ padding: 0.12, maxZoom: 1.15 }}
        minZoom={0.25}
        maxZoom={2}
        proOptions={{ hideAttribution: false }}
      >
        <FitViewport
          contentKey={switches
            .map((s) => key(s.resource.metadata))
            .sort()
            .join("|")}
        />
        <Background
          variant={BackgroundVariant.Dots}
          gap={20}
          color="var(--canvas-dot)"
        />
        <Controls showInteractive={false} />
      </ReactFlow>
      {switches.length === 0 && (
        <div className="graph-empty">
          No switches match the current filters.
        </div>
      )}
      <div className="graph-caption">
        {physical
          ? "Observed LLDP evidence · current eligible samples only"
          : "Roles from bound Cells · no physical links"}
      </div>
    </div>
  );
}
