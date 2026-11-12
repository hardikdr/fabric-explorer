import { useEffect, useState } from "react";
export type ThemePreference = "system" | "light" | "dark";
export const themeKey = "fabric-explorer-theme";
export function readPreference(): ThemePreference {
  try {
    const v = localStorage.getItem(themeKey);
    if (v === "light" || v === "dark") return v;
  } catch {
    /* Restricted storage still supports session themes. */
  }
  return "system";
}
export function ThemeSelector() {
  const [preference, setPreference] = useState<ThemePreference>(readPreference);
  useEffect(() => {
    const media = matchMedia("(prefers-color-scheme: dark)");
    const apply = () => {
      const resolved =
        preference === "system"
          ? media.matches
            ? "dark"
            : "light"
          : preference;
      document.documentElement.dataset.theme = resolved;
      document.documentElement.style.colorScheme = resolved;
    };
    apply();
    media.addEventListener("change", apply);
    try {
      localStorage.setItem(themeKey, preference);
    } catch {
      /* Storage is optional. */
    }
    return () => media.removeEventListener("change", apply);
  }, [preference]);
  return (
    <label className="theme-selector">
      <span>Theme</span>
      <select
        aria-label="Theme preference"
        value={preference}
        onChange={(e) => setPreference(e.target.value as ThemePreference)}
      >
        <option value="system">System</option>
        <option value="light">Light</option>
        <option value="dark">Dark</option>
      </select>
    </label>
  );
}
