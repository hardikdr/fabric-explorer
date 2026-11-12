import { renderHook, waitFor, cleanup } from "@testing-library/react";
import { afterEach, it, expect, vi } from "vitest";
import fixture from "../tests/fixtures/inventory.json";
import { useInventory } from "./api";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
it("accepts diagnostic inventory body on 503", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: false,
      json: async () => ({ ...fixture, switches: [] }),
    }),
  );
  const { result } = renderHook(useInventory);
  await waitFor(() => expect(result.current.loading).toBe(false));
  expect(result.current.data?.switches).toHaveLength(0);
  expect(result.current.error).toContain("No usable resource snapshot");
});
it("reports transport failure without invented inventory", async () => {
  vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("Disconnected")));
  const { result } = renderHook(useInventory);
  await waitFor(() => expect(result.current.loading).toBe(false));
  expect(result.current.data).toBeUndefined();
  expect(result.current.error).toBe("Disconnected");
});
