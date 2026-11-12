import { test, expect } from "@playwright/test";

// This suite starts only fixture mode and creates an isolated browser profile.
test("fixture inventory, topology selection, search, configuration and themes", async ({
  page,
  request,
}) => {
  await page.goto("/");
  await expect(page.getByText("FIXTURE DATA", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: /in topology$/ })).toHaveCount(
    4,
  );
  await expect(page.locator(".react-flow__edge")).toHaveCount(0);

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
  // Opening the inspector resizes the canvas; all cards must refit inside it.
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
  await inspector
    .getByRole("tab", { name: "Cell config", exact: true })
    .click();
  await expect(
    inspector.getByText("10.1.0.0/16", { exact: true }).first(),
  ).toBeVisible();
  await inspector.getByRole("tab", { name: "ports", exact: true }).click();
  await expect(
    inspector.locator("summary").filter({ hasText: "sonic://Ethernet0" }),
  ).toBeVisible();
  await inspector.getByRole("tab", { name: "evidence", exact: true }).click();
  await expect(inspector.getByText(/declared-bootstrap/)).toBeVisible();

  await page.getByRole("textbox", { name: "Search switches" }).fill("leaf-2");
  await expect(page.getByRole("button", { name: /in topology$/ })).toHaveCount(
    1,
  );
  await page
    .getByRole("button", { name: "Inspect leaf-2.clos.example", exact: true })
    .click();
  await expect(
    inspector.getByRole("heading", {
      name: "leaf-2.clos.example",
      exact: true,
    }),
  ).toBeVisible();
  await page.getByRole("textbox", { name: "Search switches" }).fill("");
  await page
    .getByRole("combobox", { name: "Filter by role" })
    .selectOption("spine");
  await expect(page.getByRole("button", { name: /in topology$/ })).toHaveCount(
    2,
  );
  await page.getByRole("combobox", { name: "Filter by role" }).selectOption("");
  await page.getByRole("button", { name: "Close inspector" }).click();

  await page
    .getByRole("combobox", { name: "Theme preference" })
    .selectOption("dark");
  await page.reload();
  await expect(
    page.getByRole("combobox", { name: "Theme preference" }),
  ).toHaveValue("dark");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.screenshot({
    path: "test-results/fixture-dark.png",
    fullPage: true,
  });
  await page
    .getByRole("combobox", { name: "Theme preference" })
    .selectOption("light");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");

  const response = await request.get("/api/v1/observations");
  expect(await response.json()).toMatchObject({
    mode: "disabled",
    switches: [],
    links: [],
  });
  const denied = await request.post("/api/v1/inventory");
  expect(denied.status()).toBe(405);
});
