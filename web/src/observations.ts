import { useEffect, useState } from "react";
export interface Domain<T> {
  state: string;
  complete: boolean;
  sampledAt?: string;
  lastAttemptAt?: string;
  error?: string;
  command?: string;
  data: T | null;
}
export interface ObservedPort {
  name: string;
  alias?: string;
  admin: string;
  oper: string;
  speed?: string;
  mtu?: number;
  fec?: string;
}
export interface BGPSession {
  neighbor: string;
  vrf: string;
  addressFamily: string;
  localInterface?: string;
  remoteASN?: number;
  state: string;
  uptime?: string;
  receivedPrefixes?: number;
  advertisedPrefixes?: number;
}
export interface LLDPNeighbor {
  localPort: string;
  chassisID?: string;
  chassisSubtype?: string;
  portID: string;
  portSubtype?: string;
  systemName?: string;
  reportedAge?: string;
  advertisedTTLSeconds?: number;
}
export interface ObservedSwitch {
  target: {
    resourceName: string;
    address: string;
    expectedMAC: string;
    expectedHostname?: string;
  };
  switchUID?: string;
  identityState: string;
  identityEvidence: string;
  device: Domain<{
    mac: string;
    hostname?: string;
    version?: string;
    platform?: string;
  }>;
  ports: Domain<ObservedPort[]>;
  addresses: Domain<{ name: string; address: string; family: string }[]>;
  bgp: Domain<BGPSession[]>;
  bgpDetails?: Domain<BGPSession[]>;
  lldp: Domain<LLDPNeighbor[]>;
  vlans: Domain<{ id: number; member?: string; tagging?: string }[]>;
}
export interface PhysicalLink {
  source: string;
  target: string;
  sourcePort: string;
  targetPort: string;
  evidence: string;
  sampledAt: string;
}
export interface Observations {
  schemaVersion: string;
  mode: string;
  pollSeconds: number;
  staleAfterSeconds: number;
  generatedAt: string;
  switches: ObservedSwitch[];
  links: PhysicalLink[];
}
export function useObservations() {
  const [data, setData] = useState<Observations>();
  const [error, setError] = useState("");
  useEffect(() => {
    let disposed = false;
    let timer: ReturnType<typeof setTimeout>;
    let controller: AbortController;
    const load = async () => {
      controller = new AbortController();
      const deadline = setTimeout(() => controller.abort(), 8000);
      let delay = 10000;
      try {
        const res = await fetch("/api/v1/observations", {
          signal: controller.signal,
        });
        if (!res.ok) throw Error("SONiC observation API unavailable");
        const body: Observations = await res.json();
        if (
          body.schemaVersion !== "v1" ||
          !Array.isArray(body.switches) ||
          !Array.isArray(body.links)
        )
          throw Error("Unsupported observation response");
        if (!disposed) {
          setData(body);
          setError("");
          delay = Math.max(
            5000,
            Math.min(30000, (body.pollSeconds || 30) * 1000),
          );
        }
      } catch {
        if (!disposed)
          setError("SONiC refresh failed; retained samples may be stale");
      } finally {
        clearTimeout(deadline);
        if (!disposed) timer = setTimeout(load, delay);
      }
    };
    const clock = setInterval(() => {
      if (!disposed) setData((current) => (current ? { ...current } : current));
    }, 5000);
    void load();
    return () => {
      disposed = true;
      clearInterval(clock);
      controller?.abort();
      clearTimeout(timer);
    };
  }, []);
  return { data, error };
}
export function domainState(
  domain: Domain<unknown>,
  limit: number,
  now = Date.now(),
): string {
  if (
    domain.sampledAt &&
    now - Date.parse(domain.sampledAt) > limit * 1000 &&
    (domain.state === "ready" || domain.state === "partial")
  )
    return "stale";
  return domain.state || "unavailable";
}

// Recheck cached edge eligibility in the browser as time passes or API reads fail.
export function eligibleLinks(
  snapshot?: Observations,
  now = Date.now(),
): PhysicalLink[] {
  if (!snapshot || snapshot.mode !== "observed") return [];
  const eligible = new Set(
    snapshot.switches
      .filter(
        (s) =>
          s.identityState === "verified" &&
          s.switchUID &&
          [s.device, s.ports, s.lldp].every(
            (d) =>
              domainState(d, snapshot.staleAfterSeconds, now) === "ready" &&
              d.complete,
          ),
      )
      .map((s) => s.switchUID),
  );
  return snapshot.links.filter(
    (l) =>
      eligible.has(l.source) &&
      eligible.has(l.target) &&
      now - Date.parse(l.sampledAt) <= snapshot.staleAfterSeconds * 1000,
  );
}
