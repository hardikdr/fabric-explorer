import { test, expect } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";

test("capture a synthetic-only README tour", async ({ page, request }) => {
  test.skip(
    process.env.CAPTURE_README_DEMO !== "1",
    "Opt-in documentation capture",
  );
  test.setTimeout(120_000);
  await page.setViewportSize({ width: 1600, height: 960 });
  const inventory = await (await request.get("/api/v1/inventory")).json();
  expect(inventory.mode).toBe("fixture");
  expect(inventory.switches).toHaveLength(4);
  expect(
    inventory.switches.every((s: { displayName: string }) =>
      s.displayName.endsWith(".example"),
    ),
  ).toBe(true);
  const output = path.join("test-results", "readme-demo");
  await mkdir(output, { recursive: true });
  const frames: { file: string; title: string; duration: number }[] = [];
  const capture = async (title: string, duration = 2200) => {
    await expect
      .poll(() =>
        page.locator(".flow").evaluate((canvas) => {
          const bounds = canvas.getBoundingClientRect();
          return Array.from(
            canvas.querySelectorAll(".react-flow__node-switch"),
          ).every((node) => {
            const card = node.getBoundingClientRect();
            return (
              card.left >= bounds.left - 1 &&
              card.right <= bounds.right + 1 &&
              card.top >= bounds.top - 1 &&
              card.bottom <= bounds.bottom + 1
            );
          });
        }),
      )
      .toBe(true);
    const file = String(frames.length + 1).padStart(2, "0") + ".png";
    await page.screenshot({
      path: path.join(output, file),
      animations: "disabled",
    });
    frames.push({ file, title, duration });
  };
  const workspace = async () => {
    await page.locator(".toolbar").evaluate((element) => {
      const top = element.getBoundingClientRect().top + window.scrollY;
      window.scrollTo({ top: Math.max(0, top - 80), behavior: "instant" });
    });
  };
  await page.goto("/");
  await expect(page.getByText("FIXTURE DATA", { exact: true })).toBeVisible();
  await page
    .getByRole("combobox", { name: "Theme preference" })
    .selectOption("dark");
  await expect(page.getByRole("button", { name: /in topology$/ })).toHaveCount(
    4,
  );
  await capture("Explore the logical fabric", 2600);

  await page
    .getByRole("button", {
      name: "Inspect leaf-1.clos.example in topology",
      exact: true,
    })
    .click();
  const inspector = page.getByLabel("Switch inspector");
  await expect(
    inspector.getByRole("heading", {
      name: "leaf-1.clos.example",
      exact: true,
    }),
  ).toBeVisible();
  await workspace();
  await capture("Select a switch and inspect its resources");

  await inspector
    .getByRole("tab", { name: "Cell config", exact: true })
    .click();
  await expect(
    inspector.getByText("10.1.0.0/16", { exact: true }).first(),
  ).toBeVisible();
  await workspace();
  await capture("Inspect Cell addresses, prefixes and local peers", 2800);

  await inspector.getByRole("tab", { name: "ports", exact: true }).click();
  await inspector
    .locator("summary")
    .filter({ hasText: "sonic://Ethernet0" })
    .click();
  await workspace();
  await capture("Compare declared ports with Fabric Interfaces");

  await inspector.getByRole("tab", { name: "evidence", exact: true }).click();
  await expect(
    inspector.getByText("declared-bootstrap", { exact: true }),
  ).toBeVisible();
  await workspace();
  await capture("Understand association evidence and diagnostics");

  await page.getByRole("textbox", { name: "Search switches" }).fill("leaf-2");
  await expect(page.getByRole("button", { name: /in topology$/ })).toHaveCount(
    1,
  );
  await page
    .getByRole("button", {
      name: "Inspect leaf-2.clos.example in topology",
      exact: true,
    })
    .click();
  await expect(
    inspector.getByRole("heading", {
      name: "leaf-2.clos.example",
      exact: true,
    }),
  ).toBeVisible();
  await workspace();
  await capture("Search the fleet with shared selection");

  await page.getByRole("textbox", { name: "Search switches" }).fill("");
  await page.getByRole("button", { name: "Close inspector" }).click();
  await page
    .getByRole("combobox", { name: "Filter by role" })
    .selectOption("spine");
  await expect(page.getByRole("button", { name: /in topology$/ })).toHaveCount(
    2,
  );
  await workspace();
  await capture("Filter by spine, leaf or unclassified role");

  await page.getByRole("combobox", { name: "Filter by role" }).selectOption("");
  await page
    .getByRole("combobox", { name: "Theme preference" })
    .selectOption("light");
  await page.evaluate(() => window.scrollTo({ top: 0, behavior: "instant" }));
  await capture("Choose System, Light or Dark", 2600);
  await writeFile(
    path.join(output, "frames.json"),
    JSON.stringify(frames, null, 2),
  );
});
