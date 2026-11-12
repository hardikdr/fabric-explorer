import {
  fireEvent,
  render,
  screen,
  within,
  cleanup,
} from "@testing-library/react";
import { afterEach, it, expect, vi } from "vitest";
import fixture from "../tests/fixtures/inventory.json";
import { Dashboard } from "./App";
import { key, filterSwitches, type Inventory } from "./model";
import { layout } from "./layout";
import { CellDetail } from "./Inspector";
const data = fixture as unknown as Inventory;
vi.mock("./Topology", () => ({
  Topology: ({
    switches,
    onSelect,
  }: {
    switches: Inventory["switches"];
    onSelect: (id: string) => void;
  }) => (
    <div aria-label="Test topology">
      {switches.map((s) => (
        <button
          key={key(s.resource.metadata)}
          onClick={() => onSelect(key(s.resource.metadata))}
        >
          Graph {s.displayName}
        </button>
      ))}
    </div>
  ),
}));
afterEach(() => {
  cleanup();
  history.replaceState(null, "", "/");
});
it("selects from table and topology; inspects configuration; preserves selection across refresh", () => {
  const { rerender } = render(<Dashboard data={data} />);
  fireEvent.click(
    screen.getByRole("button", { name: "Inspect leaf-1.clos.example" }),
  );
  let inspector = within(screen.getByLabelText("Switch inspector"));
  expect(
    inspector.getByRole("heading", { name: "leaf-1.clos.example" }),
  ).toBeInTheDocument();
  expect(inspector.getByText("sonic://leaf-1")).toBeInTheDocument();
  expect(
    inspector.getByText(/Bootstrap completion and device reachability/),
  ).toBeInTheDocument();
  fireEvent.click(inspector.getByRole("tab", { name: "Cell config" }));
  expect(inspector.getAllByText("10.1.0.0/16").length).toBeGreaterThan(0);
  expect(inspector.getAllByText("10.1.1.0/24").length).toBeGreaterThan(0);
  rerender(<Dashboard data={{ ...data, revision: data.revision + 1 }} />);
  expect(
    within(screen.getByLabelText("Switch inspector")).getByRole("tab", {
      name: "Cell config",
    }),
  ).toHaveAttribute("aria-selected", "true");
  fireEvent.click(
    screen.getByRole("button", { name: "Graph spine-2.clos.example" }),
  );
  inspector = within(screen.getByLabelText("Switch inspector"));
  expect(
    inspector.getByRole("heading", { name: "spine-2.clos.example" }),
  ).toBeInTheDocument();
  fireEvent.click(inspector.getByRole("tab", { name: "evidence" }));
  expect(inspector.getByText("declared-bootstrap")).toBeInTheDocument();
  expect(inspector.getByText("naming-convention")).toBeInTheDocument();
});
it("filters inventory and topology together", () => {
  render(<Dashboard data={data} />);
  fireEvent.change(screen.getByLabelText("Search switches"), {
    target: { value: "192.0.2.21" },
  });
  expect(
    screen.getByRole("button", { name: "Inspect leaf-1.clos.example" }),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Inspect leaf-2.clos.example" }),
  ).not.toBeInTheDocument();
  expect(
    screen.getByRole("button", { name: "Graph leaf-1.clos.example" }),
  ).toBeInTheDocument();
  fireEvent.change(screen.getByLabelText("Filter by role"), {
    target: { value: "spine" },
  });
  expect(screen.getByText("No matching switches")).toBeInTheDocument();
});
it("shows stale, unknown, and empty status honestly", () => {
  const stale = structuredClone(data);
  stale.sourceStates[0].state = "stale";
  stale.sourceStates[0].complete = false;
  stale.sourceStates[0].error = "forbidden";
  stale.switches[0].resource.status = { conditions: [] };
  stale.switches[0].role = "unknown";
  render(<Dashboard data={stale} />);
  expect(screen.getByRole("status")).toHaveTextContent(
    "Inventory is incomplete or stale",
  );
  fireEvent.click(
    screen.getByRole("button", {
      name: "Inspect " + stale.switches[0].displayName,
    }),
  );
  expect(
    within(screen.getByLabelText("Switch inspector")).getAllByText("Unknown")
      .length,
  ).toBeGreaterThan(0);
  expect(
    screen.getAllByText("No conditions reported by this resource.").length,
  ).toBeGreaterThan(0);
});
it("keeps unresolved peers and ordered prefixes visible", () => {
  const c = structuredClone(
    data.cells.find((c) => c.resource.spec.nodeRef.name === "leaf-1")!,
  );
  c.binding = "unbound";
  c.peers[2].interfaceKey = undefined;
  c.peers[2].resolution = "wrong-node";
  render(<CellDetail cell={c} data={data} />);
  expect(screen.getByText("unbound")).toBeInTheDocument();
  expect(screen.getByText("wrong-node")).toBeInTheDocument();
  expect(screen.getAllByText("10.1.1.0/24").length).toBeGreaterThan(0);
  expect(screen.getAllByText("Unknown").length).toBeGreaterThan(0);
});
it("retains selected switch when filter hides it", () => {
  render(<Dashboard data={data} />);
  fireEvent.click(
    screen.getByRole("button", { name: "Inspect leaf-1.clos.example" }),
  );
  fireEvent.change(screen.getByLabelText("Search switches"), {
    target: { value: "spine" },
  });
  expect(screen.getByLabelText("Switch inspector")).toHaveTextContent(
    "leaf-1.clos.example",
  );
});
it("distinguishes a complete empty list", () => {
  render(<Dashboard data={{ ...data, switches: [] }} />);
  expect(screen.getByText("No Switch resources listed")).toBeInTheDocument();
});
it("lays out deterministic spine, leaf, unknown bands", () => {
  const nodes = layout(data.switches, "");
  expect(nodes).toEqual(layout(data.switches, ""));
  const spines = nodes.filter(
    (n) =>
      n.type === "switch" &&
      (n.data.sw as Inventory["switches"][0]).role === "spine",
  );
  const leaves = nodes.filter(
    (n) =>
      n.type === "switch" &&
      (n.data.sw as Inventory["switches"][0]).role === "leaf",
  );
  expect(Math.max(...spines.map((n) => n.position.y))).toBeLessThan(
    Math.min(...leaves.map((n) => n.position.y)),
  );
  expect(nodes.every((n) => n.draggable === false)).toBe(true);
});
it("filters namespace and phase through targeted Cells", () => {
  expect(
    filterSwitches(data, {
      search: "",
      role: "leaf",
      association: "associated",
      namespace: "fabric-lab",
      phase: "Active",
    }),
  ).toHaveLength(2);
});
