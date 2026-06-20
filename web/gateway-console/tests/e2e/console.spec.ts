import { expect, test } from "@playwright/test";

const routes = [
  ["/ui-system", "Codencer Gateway Console UI"],
  ["/console", "Self-host bridge status"],
  ["/console/relays", "Gateway routing backends"],
  ["/console/connectors", "Local execution endpoints"],
  ["/console/projects", "Project locations"],
  ["/console/activation", "Gateway-first setup"],
  ["/console/audit", "Workspace event stream"],
  ["/console/settings", "Console settings"],
  ["/device", "Codencer approval"],
  ["/oauth/authorize", "Authorize Gateway MCP access"],
] as const;

for (const [route, heading] of routes) {
  test(`${route} renders`, async ({ page }) => {
    await page.goto(route);
    await expect(page.getByRole("heading", { name: heading })).toBeVisible();
    const html = await page.content();
    expect(html).not.toContain("official-relay-token");
    expect(html).not.toContain("selfhost-relay-token");
    expect(html).not.toMatch(/\/Users\/[^<"'\s]+/);
    expect(html).not.toMatch(/\/home\/[^<"'\s]+/);
  });
}

test("theme toggle changes theme", async ({ page }) => {
  await page.goto("/console");
  await page.getByRole("button", { name: /switch to dark theme/i }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
});

test("device form validation works", async ({ page }) => {
  await page.goto("/device");
  await page.getByRole("button", { name: /approve device/i }).click();
  await expect(page.locator("#user-code-message")).toBeVisible();
  await page.getByLabel(/user code/i).fill("ABCD-EFGH");
  await page.getByRole("button", { name: /approve device/i }).click();
  await expect(page.getByText(/device login approved/i)).toBeVisible();
});

test("oauth approve and deny controls exist", async ({ page }) => {
  await page.goto("/oauth/authorize");
  await expect(page.getByRole("button", { name: /^approve$/i })).toBeVisible();
  await expect(page.getByRole("button", { name: /^deny$/i })).toBeVisible();
});

test("keyboard navigation opens dialog and dropdown", async ({ page }) => {
  await page.goto("/ui-system");
  await page.getByTestId("dialog-open").focus();
  await page.keyboard.press("Enter");
  await expect(page.getByText(/focus is trapped/i)).toBeVisible();
  await page.getByRole("button", { name: /close dialog/i }).focus();
  await page.keyboard.press("Enter");
  await expect(page.getByText(/focus is trapped/i)).toBeHidden();
  await page.getByTestId("dropdown-open").press("Enter");
  await expect(
    page.getByRole("menuitem", { name: /copy command/i }),
  ).toBeVisible();
});
