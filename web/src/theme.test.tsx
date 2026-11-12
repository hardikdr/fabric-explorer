import boot from "../public/theme-init.js?raw";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
} from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ThemeSelector, themeKey } from "./theme";
let dark = false;
let listeners: Set<() => void>;
beforeEach(() => {
  const storage = new Map<string, string>();
  vi.stubGlobal("localStorage", {
    getItem: (k: string) => storage.get(k) ?? null,
    setItem: (k: string, v: string) => storage.set(k, v),
    clear: () => storage.clear(),
  });
  dark = false;
  listeners = new Set();
  vi.stubGlobal("matchMedia", () => ({
    get matches() {
      return dark;
    },
    addEventListener: (_: string, fn: () => void) => listeners.add(fn),
    removeEventListener: (_: string, fn: () => void) => listeners.delete(fn),
  }));
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
it("defaults to System and responds to OS changes", () => {
  render(<ThemeSelector />);
  expect(screen.getByLabelText("Theme preference")).toHaveValue("system");
  expect(document.documentElement.dataset.theme).toBe("light");
  act(() => {
    dark = true;
    listeners.forEach((fn) => fn());
  });
  expect(document.documentElement.dataset.theme).toBe("dark");
});
it("persists manual preference and ignores OS changes until System is selected", () => {
  const view = render(<ThemeSelector />);
  fireEvent.change(screen.getByLabelText("Theme preference"), {
    target: { value: "dark" },
  });
  expect(localStorage.getItem(themeKey)).toBe("dark");
  view.unmount();
  render(<ThemeSelector />);
  expect(screen.getByLabelText("Theme preference")).toHaveValue("dark");
  act(() => {
    dark = false;
    listeners.forEach((fn) => fn());
  });
  expect(document.documentElement.dataset.theme).toBe("dark");
  fireEvent.change(screen.getByLabelText("Theme preference"), {
    target: { value: "system" },
  });
  expect(document.documentElement.dataset.theme).toBe("light");
});
it("ignores invalid stored preferences and removes media subscriptions", () => {
  localStorage.setItem(themeKey, "invalid");
  const view = render(<ThemeSelector />);
  expect(screen.getByLabelText("Theme preference")).toHaveValue("system");
  view.unmount();
  expect(listeners.size).toBe(0);
});

it("applies persisted and system themes before React loads", () => {
  localStorage.setItem(themeKey, "dark");
  new Function(boot)();
  expect(document.documentElement.dataset.theme).toBe("dark");
  localStorage.setItem(themeKey, "light");
  dark = true;
  new Function(boot)();
  expect(document.documentElement.dataset.theme).toBe("light");
  localStorage.setItem(themeKey, "system");
  new Function(boot)();
  expect(document.documentElement.dataset.theme).toBe("dark");
});
