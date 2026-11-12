import { useEffect, useState } from "react";
import type { Inventory } from "./model";
export function useInventory() {
  const [data, setData] = useState<Inventory>();
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    let disposed = false;
    let inFlight = false;
    let timer: ReturnType<typeof setTimeout>;
    let controller: AbortController;
    const load = async () => {
      if (disposed || inFlight) return;
      if (document.hidden) {
        timer = setTimeout(load, 15000);
        return;
      }
      inFlight = true;
      controller = new AbortController();
      const deadline = setTimeout(() => controller.abort(), 8000);
      let delay = 5000;
      try {
        const res = await fetch("/api/v1/inventory", {
          signal: controller.signal,
        });
        const body: Inventory = await res.json();
        if (
          body.schemaVersion !== "v1" ||
          !Array.isArray(body.switches) ||
          !Array.isArray(body.sourceStates)
        )
          throw new Error("Unsupported inventory response");
        if (!disposed) {
          setData(body);
          setError(
            res.ok
              ? ""
              : "No usable resource snapshot. Inspect source access below.",
          );
          delay = Math.max(1000, body.pollSeconds * 1000);
        }
      } catch (e) {
        if (!disposed)
          setError(e instanceof Error ? e.message : "Resource refresh failed");
      } finally {
        inFlight = false;
        clearTimeout(deadline);
        if (!disposed) {
          setLoading(false);
          timer = setTimeout(load, delay);
        }
      }
    };
    const onVisibility = () => {
      if (!document.hidden) {
        clearTimeout(timer);
        void load();
      }
    };
    document.addEventListener("visibilitychange", onVisibility);
    void load();
    return () => {
      disposed = true;
      document.removeEventListener("visibilitychange", onVisibility);
      clearTimeout(timer);
      controller?.abort();
    };
  }, []);
  return { data, error, loading };
}
