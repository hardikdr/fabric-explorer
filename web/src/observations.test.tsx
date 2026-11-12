import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import { describe, it, expect, vi, afterEach } from "vitest";
import {
  BGPPanel,
  LLDPPanel,
  VLANPanel,
  ObservationFleet,
} from "./ObservedPanels";
import {
  domainState,
  eligibleLinks,
  type ObservedSwitch,
} from "./observations";
import { Inspector } from "./Inspector";
import type { Inventory } from "./model";
import fixture from "../tests/fixtures/inventory.json";

afterEach(() => cleanup());
const inventory = fixture as unknown as Inventory;
function observation(): ObservedSwitch {
  const sample = new Date().toISOString();
  const domain = <T,>(data: T) => ({
    state: "ready",
    complete: true,
    sampledAt: sample,
    data,
  });
  return {
    target: {
      resourceName: "lab-switch",
      address: "test",
      expectedMAC: "00:11:22:33:44:55",
    },
    switchUID: inventory.switches[0].resource.metadata.uid,
    identityState: "verified",
    identityEvidence: "Test-only verified MAC",
    device: domain({ mac: "00:11:22:33:44:55" }),
    ports: domain([]),
    addresses: domain([]),
    bgp: domain([
      {
        neighbor: "Ethernet120",
        vrf: "default",
        addressFamily: "ipv6-unicast",
        state: "Idle",
        remoteASN: 4200000001,
      },
    ]),
    lldp: domain([
      {
        localPort: "Ethernet120",
        portID: "Eth1",
        systemName: "peer",
        chassisID: "00:11:22:33:44:66",
        chassisSubtype: "mac",
        portSubtype: "local",
      },
    ]),
    vlans: domain([]),
  };
}
describe("operational samples", () => {
  it("expires timestamps and preserves unavailable states", () => {
    expect(
      domainState(
        {
          state: "ready",
          complete: true,
          sampledAt: "2020-01-01T00:00:00Z",
          data: [],
        },
        90,
      ),
    ).toBe("stale");
    expect(
      domainState({ state: "unavailable", complete: false, data: null }, 90),
    ).toBe("unavailable");
  });
  it("shows real BGP states without inferring established", () => {
    render(<BGPPanel observation={observation()} limit={90} />);
    expect(screen.getByText("Idle")).toBeInTheDocument();
    expect(screen.getByText("Ethernet120")).toBeInTheDocument();
    expect(screen.queryByText("Established")).not.toBeInTheDocument();
    expect(screen.getByText(/Received Not reported/)).toBeInTheDocument();
  });
  it("shows LLDP identity evidence and VLAN empty semantics", () => {
    const o = observation();
    render(
      <>
        <LLDPPanel observation={o} limit={90} />
        <VLANPanel observation={o} limit={90} />
      </>,
    );
    expect(screen.getByText(/00:11:22:33:44:66/)).toBeInTheDocument();
    expect(screen.getByText(/No VLAN entries returned/)).toBeInTheDocument();
    expect(
      screen.getByText(/forwarding state was not measured/),
    ).toBeInTheDocument();
  });
  it("never attaches observations from a different Switch UID", () => {
    const o = observation();
    o.switchUID = "wrong-device";
    render(
      <Inspector
        data={inventory}
        sw={inventory.switches[0]}
        observation={o}
        observationMode="observed"
        observationLimit={90}
        onClose={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("tab", { name: /BGP Sessions/i }));
    expect(screen.queryByText("Ethernet120")).not.toBeInTheDocument();
    expect(screen.queryByText("Idle")).not.toBeInTheDocument();
  });
  it("identifies replay data and failed collection", () => {
    const o = observation();
    o.identityState = "unavailable";
    o.identityEvidence = "authentication";
    render(
      <ObservationFleet
        data={{
          schemaVersion: "v1",
          mode: "replay",
          pollSeconds: 30,
          staleAfterSeconds: 90,
          generatedAt: new Date().toISOString(),
          switches: [o],
          links: [],
        }}
        error="Refresh failed"
      />,
    );
    expect(screen.getByText("CAPTURE REPLAY")).toBeInTheDocument();
    expect(screen.getByText("authentication")).toBeInTheDocument();
  });
});

it("suppresses cached physical links after expiry or identity conflict", () => {
  const a = observation(),
    b = observation();
  b.switchUID = "peer-uid";
  const snapshot = {
    schemaVersion: "v1",
    mode: "observed",
    pollSeconds: 30,
    staleAfterSeconds: 90,
    generatedAt: new Date().toISOString(),
    switches: [a, b],
    links: [
      {
        source: a.switchUID!,
        target: b.switchUID!,
        sourcePort: "Ethernet120",
        targetPort: "Ethernet0",
        evidence: "reciprocal LLDP",
        sampledAt: new Date().toISOString(),
      },
    ],
  };
  expect(eligibleLinks(snapshot)).toHaveLength(1);
  expect(eligibleLinks(snapshot, Date.now() + 91000)).toHaveLength(0);
  b.identityState = "mismatch";
  expect(eligibleLinks(snapshot)).toHaveLength(0);
});
