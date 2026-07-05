import { expect, test } from "@playwright/test";

test.describe.configure({ mode: "serial" });

const routes = [
  ["/ui-system", "Codencer Gateway Console UI"],
  ["/console", "Self-host bridge status"],
  ["/console/relays", "Gateway routing backends"],
  ["/console/connectors", "Local execution endpoints"],
  ["/console/projects", "Project locations"],
  ["/console/runs", "Run history"],
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
    const visibleText = await page.locator("body").innerText();
    expect(visibleText).not.toContain("official-relay-token");
    expect(visibleText).not.toContain("selfhost-relay-token");
    expect(visibleText).not.toMatch(/\/Users\/[^<"'\s]+/);
    expect(visibleText).not.toMatch(/\/home\/[^<"'\s]+/);
  });
}

test("theme toggle changes theme", async ({ page }) => {
  await page.goto("/console");
  await page.getByRole("button", { name: /switch to dark theme/i }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
});

test("project task form submits demo run without unsafe output", async ({
  page,
}) => {
  await page.goto("/console/projects");
  const executor = page.getByRole("combobox", { name: "Executor" });
  await expect(executor).toContainText("codex-workspace");
  await expect(page.getByLabel(/title/i)).toHaveValue(
    "Codex workspace smoke task",
  );
  await expect(page.getByLabel(/goal/i)).toHaveValue(
    "Inspect the project README and return a short summary. Do not modify files.",
  );
  await expect(page.getByLabel(/title/i)).not.toHaveValue(
    "Gateway Console fake-safe task",
  );
  await expect(page.getByText("Route preview")).toBeVisible();
  await expect(page.locator("dt", { hasText: "Executor" })).toBeVisible();
  await expect(page.getByText("codex-workspace").first()).toBeVisible();
  await expect(page.getByLabel(/timeout/i)).toHaveValue("300");
  await expect(page.getByLabel(/manifest \/ run plan/i)).toBeHidden();
  await page.getByRole("button", { name: /^submit$/i }).click();
  await expect(page.getByText(/^Result$/i)).toBeVisible();
  await expect(page.getByText("Real executor")).toBeVisible();
  await expect(page.getByText(/run_demo_console/i)).toBeVisible();
  await expect(page.getByText(/README summary/i)).toBeVisible();
  await expect(page.getByText(/local\/self-host bridge/i)).toBeVisible();
  const fullRunLink = page.getByRole("link", { name: /view full run/i });
  await expect(fullRunLink).toBeVisible();
  await Promise.all([
    page.waitForURL(/\/console\/runs\/runhist_demo_console$/),
    fullRunLink.click(),
  ]);
  await expect(page.getByRole("heading", { name: /full run/i })).toBeVisible();
  await expect(page.getByText("Real executor").first()).toBeVisible();
  await expect(page.getByText(/run_demo_console/i).first()).toBeVisible();
  await expect(page.getByText(/event timeline/i)).toBeVisible();
  await expect(page.getByText(/task_submitted/i)).toBeVisible();
  await page.goto("/console/runs");
  const runsTable = page.getByRole("table");
  await expect(
    runsTable.getByText(/Codex workspace smoke task/i),
  ).toBeVisible();
  await expect(runsTable.getByText(/Claude CLI smoke task/i)).toBeVisible();
  await expect(runsTable.getByText("claude-default")).toBeVisible();
  await expect(runsTable.getByText("real").first()).toBeVisible();
  await expect(runsTable.getByText(/Codencer bridges planners/i)).toBeVisible();
  await expect(
    page.getByText(/records structured runs, results, blockers/i),
  ).toBeHidden();
  const visibleText = await page.locator("body").innerText();
  expect(visibleText).not.toMatch(/\/Users\/|\/tmp\/|\/var\/folders\//);
  expect(visibleText).not.toContain("report_path");
  expect(visibleText).not.toContain("logs_ref");
});

test("project task form validates executor selection", async ({ page }) => {
  await page.goto("/console/projects");
  await page.getByRole("button", { name: /^advanced$/i }).click();
  await page
    .getByLabel(/manual executor profile override/i)
    .fill("not-a-real-executor");
  await expect(page.getByText(/unknown executor profile/i)).toBeVisible();
  await expect(page.getByRole("button", { name: /^submit$/i })).toBeDisabled();
});

test("elevated Codex executor requires confirmation", async ({ page }) => {
  await page.goto("/console/projects");
  await page.getByRole("combobox", { name: "Executor" }).click();
  await page.getByRole("option", { name: /codex-full/i }).click();
  await expect(
    page.getByText(/codex-full requires explicit confirmation/i),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: /^submit$/i })).toBeDisabled();
  await page.getByLabel(/confirm elevated executor/i).click();
  await expect(page.getByRole("button", { name: /^submit$/i })).toBeEnabled();
});

test("Antigravity executor uses real-executor defaults", async ({ page }) => {
  await page.goto("/console/projects");
  await page.getByRole("combobox", { name: "Executor" }).click();
  await page.getByRole("option", { name: /antigravity-default/i }).click();
  await expect(page.getByLabel(/title/i)).toHaveValue("Antigravity smoke task");
  await expect(page.getByLabel(/goal/i)).toHaveValue(
    "Inspect the project README and return a short summary. Do not modify files.",
  );
  await expect(page.getByLabel(/title/i)).not.toHaveValue(
    "Gateway Console fake-safe task",
  );
  await expect(page.getByLabel(/timeout/i)).toHaveValue("300");
  await page.getByRole("button", { name: /^advanced$/i }).click();
  await page.getByRole("combobox", { name: /execution mode/i }).click();
  await page.getByRole("option", { name: /manifest \/ run plan/i }).click();
  await expect(page.getByLabel(/manifest \/ run plan/i)).toHaveValue(
    /profile: antigravity-default/,
  );
});

test("manifest run plan is advanced and guided", async ({ page }) => {
  await page.goto("/console/projects");
  await expect(page.getByLabel(/manifest \/ run plan/i)).toBeHidden();
  await page.getByRole("button", { name: /^advanced$/i }).click();
  await page.getByRole("combobox", { name: /execution mode/i }).click();
  await page.getByRole("option", { name: /manifest \/ run plan/i }).click();
  await expect(page.getByText(/manifest schema help/i)).toBeVisible();
  await expect(page.getByLabel(/manifest \/ run plan/i)).toHaveValue(
    /execution:/,
  );
  await expect(page.getByLabel(/manifest \/ run plan/i)).toHaveValue(
    /profile: codex-workspace/,
  );
});

test("device form validation works", async ({ page }) => {
  await page.goto("/device");
  await page.getByRole("button", { name: /approve device/i }).click();
  await expect(page.locator("#user-code-message")).toBeVisible();
  await page.getByLabel(/user code/i).fill("ABCD-EFGH");
  await page.getByRole("button", { name: /approve device/i }).click();
  await expect(page.getByText(/device login approved/i)).toBeVisible();
});

test("product navigation hides UI System", async ({ page }) => {
  await page.goto("/console");
  await expect(page.getByTestId("nav-menu")).toBeVisible();
  await expect(
    page.locator('link[rel="icon"][href="/favicon.svg"]'),
  ).toHaveCount(1);
  const sidebar = page.getByRole("complementary");
  await expect(sidebar.getByText("codencer")).toBeVisible();
  await expect(page.getByText("UI System")).toBeHidden();
  await expect(page.getByText("Runs").first()).toBeVisible();
  await page.getByRole("button", { name: /toggle sidebar/i }).click();
  await expect(sidebar.getByText("codencer")).toBeHidden();
  await expect(sidebar.getByRole("img", { name: "Codencer" })).toBeVisible();
});

test("settings and relay pages expose only public self-host operator controls", async ({
  page,
}) => {
  await page.goto("/console/settings");
  await expect(page.getByText(/Future private Cloud features/i)).toBeHidden();
  await expect(page.getByText(/Token revocation/i)).toBeHidden();
  await expect(page.getByText(/MCP endpoint/i)).toBeVisible();
  await expect(page.getByText(/Console settings/i)).toBeVisible();

  await page.goto("/console/relays");
  await expect(page.getByText(/Gateway MCP path/i)).toBeVisible();
  await expect(page.getByText("http://127.0.0.1:19090/mcp")).toBeVisible();
  await expect(page.getByRole("columnheader", { name: "ID" })).toBeVisible();
  await expect(
    page.getByRole("columnheader", { name: "Token reference" }),
  ).toBeVisible();
  await expect(
    page.getByRole("cell", { name: "Default self-host Relay" }),
  ).toBeVisible();
  await expect(page.getByText("Add self-host Relay profile")).toBeVisible();
});

test("run history and audit expose pagination and grouped lifecycle", async ({
  page,
}) => {
  await page.goto("/console/runs");
  await expect(page.getByText(/showing \d+ runs from offset 0/i)).toBeVisible();
  await expect(
    page.getByRole("columnheader", { name: "Project / Machine" }),
  ).toBeVisible();
  await expect(
    page.getByRole("columnheader", { name: "Connector" }),
  ).toBeHidden();
  await expect(
    page.getByRole("link", { name: /codex workspace smoke task/i }).first(),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: /^previous$/i }),
  ).toBeDisabled();
  await expect(page.getByRole("button", { name: /^next$/i })).toBeDisabled();

  await page.goto("/console/audit");
  await expect(page.getByText(/run lifecycle/i)).toBeVisible();
  await expect(
    page.getByText(/lifecycle events for run/i).first(),
  ).toBeVisible();
  await expect(page.getByText(/report_read x 2/i).first()).toBeVisible();
  await expect(
    page.getByRole("link", { name: /view run/i }).first(),
  ).toBeVisible();
  await expect(
    page.getByText(/showing \d+ events from offset 0/i),
  ).toBeVisible();
});

test("run detail uses compact metadata, attempts, and artifact tables", async ({
  page,
}) => {
  await page.goto("/console/runs/runhist_demo_claude");
  await expect(page.getByRole("heading", { name: /full run/i })).toBeVisible();
  await expect(page.getByText("claude").first()).toBeVisible();
  await expect(page.getByText("claude-default").first()).toBeVisible();
  await expect(page.getByText("Real executor").first()).toBeVisible();
  await expect(
    page.locator("dt", { hasText: /^Adapter$/ }).first(),
  ).toBeVisible();
  await expect(
    page.locator("dt", { hasText: /^Executor profile$/ }).first(),
  ).toBeVisible();
  await expect(
    page.locator("dt", { hasText: /^Run ID$/ }).first(),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: /^summary$/i }).first(),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: /^attempts$/i }),
  ).toBeVisible();
  await expect(
    page.getByRole("columnheader", { name: "Executor profile" }).first(),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: /safe artifacts and logs/i }),
  ).toBeVisible();
  await expect(page.getByRole("columnheader", { name: "Name" })).toBeVisible();
  await expect(page.getByRole("cell", { name: "stdout.log" })).toBeVisible();
  await expect(page.getByRole("cell", { name: "stdout.log" })).toHaveCount(1);
});

test("run detail records human interrupt response", async ({ page }) => {
  await page.goto("/console/runs/runhist_demo_blocked");
  await expect(
    page.getByRole("heading", { name: /human interrupt response/i }),
  ).toBeVisible();
  await expect(page.getByText(/waiting for human action/i)).toBeVisible();
  await expect(page.getByText(/inspect README only/i)).toBeVisible();
  await expect(page.getByText(/resume records intent only/i)).toBeVisible();
  await page
    .getByLabel(/operator response/i)
    .fill("Proceed with README-only inspection. Do not modify files.");
  await page.getByRole("button", { name: /record response/i }).click();
  await expect(page.getByText(/response recorded/i).first()).toBeVisible();
  await expect(page.getByText(/README-only inspection/i).first()).toBeVisible();
  const visibleText = await page.locator("body").innerText();
  expect(visibleText).not.toMatch(/\/Users\/|\/tmp\/|\/var\/folders\//);
  expect(visibleText).not.toContain("relay-secret");
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
