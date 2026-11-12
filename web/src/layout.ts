import type { Node } from "@xyflow/react";
import { key, type SwitchView } from "./model";
export function layout(switches: SwitchView[], selected: string): Node[] {
  const result: Node[] = [];
  let y = 0;
  const columns = Math.min(
    4,
    Math.max(
      1,
      ...["spine", "leaf", "unknown"].map(
        (r) => switches.filter((s) => s.role === r).length,
      ),
    ),
  );
  for (const role of ["spine", "leaf", "unknown"]) {
    const group = switches
      .filter((sw) => sw.role === role)
      .sort(
        (a, b) =>
          a.displayName.localeCompare(b.displayName, undefined, {
            numeric: true,
          }) ||
          key(a.resource.metadata).localeCompare(key(b.resource.metadata)),
      );
    if (!group.length) continue;
    result.push({
      id: "group-" + role,
      type: "label",
      position: { x: 0, y },
      data: {
        width: columns * 280 - 24,
        label:
          role === "unknown"
            ? "UNCLASSIFIED"
            : role === "spine"
              ? "SPINE TIER"
              : "LEAF TIER",
        count: group.length,
      },
      selectable: false,
      draggable: false,
    });
    group.forEach((sw, i) =>
      result.push({
        id: key(sw.resource.metadata),
        type: "switch",
        position: {
          x:
            ((columns - Math.min(4, group.length - Math.floor(i / 4) * 4)) *
              280) /
              2 +
            (i % 4) * 280,
          y: y + 44 + Math.floor(i / 4) * 144,
        },
        data: { sw },
        selected: key(sw.resource.metadata) === selected,
        draggable: false,
      }),
    );
    y += Math.max(1, Math.ceil(group.length / 4)) * 144 + 64;
  }
  return result;
}
