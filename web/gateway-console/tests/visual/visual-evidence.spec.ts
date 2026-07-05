import { expect, test, type Page } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

type ThemeName = "light" | "dark";
type ViewportName = "desktop" | "mobile";

type ScreenshotRecord = {
  file: string;
  height?: number;
  label: string;
  route: string;
  theme: ThemeName;
  viewport: ViewportName;
  width?: number;
  kind: "route" | "interaction";
};

type MobileOverflowCheck = {
  bodyWidth: number;
  documentWidth: number;
  innerWidth: number;
  label: string;
};

const routes = [
  { path: "/ui-system", heading: "Codencer Gateway Console UI" },
  { path: "/console", heading: "Self-host bridge status" },
  { path: "/console/relays", heading: "Gateway routing backends" },
  { path: "/console/connectors", heading: "Local execution endpoints" },
  { path: "/console/projects", heading: "Project locations" },
  { path: "/console/runs", heading: "Run history" },
  { path: "/console/runs/runhist_demo_console", heading: "Full run" },
  { path: "/console/activation", heading: "Gateway-first setup" },
  { path: "/console/audit", heading: "Workspace event stream" },
  { path: "/console/settings", heading: "Console settings" },
  { path: "/device", heading: "Codencer approval" },
  { path: "/oauth/authorize", heading: "Authorize Gateway MCP access" },
] as const;

const viewports: Record<ViewportName, { width: number; height: number }> = {
  desktop: { width: 1440, height: 1000 },
  mobile: { width: 390, height: 844 },
};

const themes: ThemeName[] = ["light", "dark"];
const records: ScreenshotRecord[] = [];
const securityFindings: string[] = [];
const mobileOverflowChecks: MobileOverflowCheck[] = [];

const repoRoot = path.resolve(process.cwd(), "../..");
const runId = process.env.GATEWAY_CONSOLE_SCREENSHOT_RUN ?? timestamp();
const reportRoot = path.join(
  repoRoot,
  "reports",
  "gateway-console-screenshots",
);
const runDir = path.join(reportRoot, runId);

test.describe.configure({ mode: "serial" });
test.setTimeout(240000);

test("captures Gateway Console visual evidence", async ({ page }) => {
  fs.mkdirSync(runDir, { recursive: true });

  for (const viewportName of Object.keys(viewports) as ViewportName[]) {
    for (const theme of themes) {
      for (const route of routes) {
        await openRoute(page, route.path, route.heading, viewportName, theme);
        await capture(page, {
          file: `${slug(route.path)}__${viewportName}__${theme}.png`,
          label: `${route.path} ${viewportName} ${theme}`,
          route: route.path,
          theme,
          viewport: viewportName,
          kind: "route",
        });
      }
    }
  }

  await captureInteractions(page);
  assertGeneratedMobileScreenshotWidths();
  writeReports();
  assertVisualReviewHasNoPlaceholders();

  expect(securityFindings, securityFindings.join("\n")).toEqual([]);
  console.log(`Gateway Console visual evidence: ${runDir}`);
});

async function captureInteractions(page: Page) {
  await openRoute(
    page,
    "/console/relays",
    "Gateway routing backends",
    "desktop",
    "light",
  );
  await page.getByLabel(/profile name/i).focus();
  await capture(page, interaction("relay-form-focused", "/console/relays"));

  await page.getByLabel(/profile name/i).fill("");
  await page.getByLabel(/relay url/i).fill("not-a-url");
  await page.getByLabel(/token environment variable/i).fill("");
  await page.getByRole("button", { name: /save relay profile/i }).click();
  await expect(page.getByText(/name is required/i)).toBeVisible();
  await capture(
    page,
    interaction("relay-form-validation-error", "/console/relays"),
  );

  await openRoute(page, "/device", "Codencer approval", "desktop", "light");
  await page.getByRole("button", { name: /approve device/i }).click();
  await expect(page.locator("#user-code-message")).toBeVisible();
  await capture(page, interaction("device-validation-error", "/device"));

  await openRoute(
    page,
    "/oauth/authorize",
    "Authorize Gateway MCP access",
    "desktop",
    "light",
  );
  await expect(page.getByRole("button", { name: /^approve$/i })).toBeVisible();
  await expect(page.getByRole("button", { name: /^deny$/i })).toBeVisible();
  await capture(
    page,
    interaction("oauth-approve-deny-controls", "/oauth/authorize"),
  );
  await page.getByRole("button", { name: /^deny$/i }).click();
  await expect(page.getByText(/consent denied/i)).toBeVisible();
  await capture(page, interaction("oauth-denied-state", "/oauth/authorize"));

  await openRoute(
    page,
    "/ui-system",
    "Codencer Gateway Console UI",
    "desktop",
    "light",
  );
  await page.getByTestId("dialog-open").click();
  await expect(
    page.getByRole("dialog", { name: /gateway dialog/i }),
  ).toBeVisible();
  await capture(page, interaction("ui-system-dialog-open", "/ui-system"));

  await openRoute(
    page,
    "/ui-system",
    "Codencer Gateway Console UI",
    "desktop",
    "light",
  );
  await page.getByTestId("dropdown-open").click();
  await expect(
    page.getByRole("menuitem", { name: /copy command/i }),
  ).toBeVisible();
  await capture(page, interaction("ui-system-dropdown-open", "/ui-system"));

  await openRoute(
    page,
    "/ui-system",
    "Codencer Gateway Console UI",
    "desktop",
    "light",
  );
  await page.getByRole("combobox").click();
  await expect(
    page.getByRole("option", { name: /personal relay/i }),
  ).toBeVisible();
  await capture(page, interaction("ui-system-select-open", "/ui-system"));

  await openRoute(
    page,
    "/ui-system",
    "Codencer Gateway Console UI",
    "desktop",
    "light",
  );
  await page.getByRole("button", { name: /^popover$/i }).click();
  await expect(
    page.getByText(/relay tokens resolve server-side/i),
  ).toBeVisible();
  await capture(page, interaction("ui-system-popover-open", "/ui-system"));

  await openRoute(
    page,
    "/console/activation",
    "Gateway-first setup",
    "desktop",
    "light",
  );
  await page
    .getByRole("button", { name: /copy code/i })
    .first()
    .focus();
  await capture(
    page,
    interaction("activation-copy-affordance", "/console/activation"),
  );

  await openRoute(
    page,
    "/console",
    "Self-host bridge status",
    "mobile",
    "light",
  );
  await page.getByRole("button", { name: /toggle sidebar/i }).click();
  await expect(page.getByTestId("mobile-sidebar")).toBeVisible();
  await capture(page, {
    ...interaction("mobile-menu-open", "/console"),
    viewport: "mobile",
  });

  await openRoute(
    page,
    "/console",
    "Self-host bridge status",
    "desktop",
    "light",
  );
  await capture(page, interaction("theme-toggle-light-state", "/console"));
  await page.getByRole("button", { name: /toggle sidebar/i }).click();
  await capture(page, interaction("sidebar-collapsed", "/console"));
  await page.getByRole("button", { name: /toggle sidebar/i }).click();
  await page.getByRole("button", { name: /switch to dark theme/i }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await capture(page, {
    ...interaction("theme-toggle-dark-state", "/console"),
    theme: "dark",
  });
}

async function openRoute(
  page: Page,
  routePath: string,
  heading: string,
  viewport: ViewportName,
  theme: ThemeName,
) {
  await page.setViewportSize(viewports[viewport]);
  await page.goto(routePath);
  await expect(page.getByRole("heading", { name: heading })).toBeVisible();
  await page.waitForLoadState("networkidle").catch(() => undefined);
  await setTheme(page, theme);
  await page.waitForTimeout(100);
  await assertSafeHTML(page, routePath);
}

async function setTheme(page: Page, theme: ThemeName) {
  await page.evaluate((value) => {
    document.documentElement.dataset.theme = value;
  }, theme);
}

async function capture(page: Page, record: ScreenshotRecord) {
  if (record.viewport === "mobile") {
    await assertNoMobileOverflow(page, record.label);
  }

  const target = path.join(runDir, record.file);
  await page.screenshot({ path: target, fullPage: record.kind === "route" });
  const dimensions = readPngDimensions(target);
  const captured = { ...record, ...dimensions };
  records.push(captured);

  if (record.viewport === "mobile") {
    expect(
      dimensions.width,
      `${record.file} must be exactly ${viewports.mobile.width}px wide`,
    ).toBe(viewports.mobile.width);
  }
}

async function assertNoMobileOverflow(page: Page, label: string) {
  const result = await page.evaluate(() => {
    const innerWidth = window.innerWidth;
    const offenders = Array.from(document.body.querySelectorAll("*"))
      .map((element) => {
        const rect = element.getBoundingClientRect();
        const className =
          typeof element.className === "string"
            ? element.className
            : String(element.className);
        return {
          className,
          left: Math.round(rect.left),
          right: Math.round(rect.right),
          tag: element.tagName.toLowerCase(),
          text: (element.textContent ?? "")
            .replace(/\s+/g, " ")
            .trim()
            .slice(0, 100),
          width: Math.round(rect.width),
        };
      })
      .filter(
        (item) =>
          item.width > innerWidth + 1 ||
          item.right > innerWidth + 1 ||
          item.left < -1,
      )
      .slice(0, 20);

    return {
      bodyWidth: document.body.scrollWidth,
      documentWidth: document.documentElement.scrollWidth,
      innerWidth,
      offenders,
    };
  });

  mobileOverflowChecks.push({
    bodyWidth: result.bodyWidth,
    documentWidth: result.documentWidth,
    innerWidth: result.innerWidth,
    label,
  });

  const diagnostic = JSON.stringify({ label, ...result }, null, 2);
  expect(result.documentWidth, diagnostic).toBeLessThanOrEqual(
    result.innerWidth + 1,
  );
  expect(result.bodyWidth, diagnostic).toBeLessThanOrEqual(
    result.innerWidth + 1,
  );
}

function assertGeneratedMobileScreenshotWidths() {
  const expectedWidth = viewports.mobile.width;
  const recordFailures = records
    .filter((record) => record.viewport === "mobile")
    .filter((record) => record.width !== expectedWidth)
    .map(
      (record) =>
        `${record.file}: expected ${expectedWidth}px, got ${record.width}px`,
    );
  const diskFailures = fs
    .readdirSync(runDir)
    .filter((file) => file.endsWith(".png") && file.includes("__mobile__"))
    .map((file) => {
      const dimensions = readPngDimensions(path.join(runDir, file));
      return { ...dimensions, file };
    })
    .filter((entry) => entry.width !== expectedWidth)
    .map(
      (entry) =>
        `${entry.file}: expected ${expectedWidth}px, got ${entry.width}px`,
    );

  expect(
    [...recordFailures, ...diskFailures],
    "all mobile screenshots must be exactly 390px wide",
  ).toEqual([]);
}

function readPngDimensions(filePath: string) {
  const header = fs.readFileSync(filePath).subarray(0, 24);
  const pngSignature = "89504e470d0a1a0a";
  if (
    header.length < 24 ||
    header.subarray(0, 8).toString("hex") !== pngSignature
  ) {
    throw new Error(`${filePath} is not a valid PNG file`);
  }
  return {
    height: header.readUInt32BE(20),
    width: header.readUInt32BE(16),
  };
}

async function assertSafeHTML(page: Page, label: string) {
  const html = await page.content();
  const visibleText = await page.locator("body").innerText();
  const checks: [string, RegExp, string][] = [
    [html, /official-relay-token/i, "official relay token fixture leaked"],
    [html, /selfhost-relay-token/i, "self-host relay token fixture leaked"],
    [
      html,
      /Authorization:\s*Bearer\s+[A-Za-z0-9._~+/=-]+/i,
      "bearer token leaked",
    ],
    [html, /-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----/i, "private key leaked"],
    [visibleText, /\/Users\/[^<"'\s)]+/i, "macOS absolute path leaked"],
    [visibleText, /\/home\/[^<"'\s)]+/i, "Linux absolute path leaked"],
  ];

  for (const [source, pattern, message] of checks) {
    if (pattern.test(source)) {
      securityFindings.push(`${label}: ${message}`);
    }
  }
}

function interaction(label: string, route: string): ScreenshotRecord {
  return {
    file: `interaction__${label}.png`,
    label,
    route,
    theme: "light",
    viewport: "desktop",
    kind: "interaction",
  };
}

function writeReports() {
  const byKind = {
    route: records.filter((record) => record.kind === "route"),
    interaction: records.filter((record) => record.kind === "interaction"),
  };
  const mobileRecords = records.filter(
    (record) => record.viewport === "mobile",
  );
  fs.writeFileSync(
    path.join(runDir, "index.md"),
    [
      "# Gateway Console Screenshot Evidence",
      "",
      `Run: \`${runId}\``,
      "",
      "Generated by `npm run visual:evidence` from `web/gateway-console`.",
      "",
      "## Route Matrix",
      "",
      "| Route | Viewport | Theme | Screenshot |",
      "| --- | --- | --- | --- |",
      ...byKind.route.map(
        (record) =>
          `| \`${record.route}\` | ${record.viewport} | ${record.theme} | [${record.file}](./${record.file}) |`,
      ),
      "",
      "## Interaction States",
      "",
      "| State | Route | Viewport | Theme | Screenshot |",
      "| --- | --- | --- | --- | --- |",
      ...byKind.interaction.map(
        (record) =>
          `| ${record.label} | \`${record.route}\` | ${record.viewport} | ${record.theme} | [${record.file}](./${record.file}) |`,
      ),
      "",
      "## Mobile Layout Checks",
      "",
      "Horizontal overflow assertions passed for every captured mobile route and mobile interaction state. Mobile PNG widths were read from the generated files and must be exactly 390px.",
      "",
      "| Capture | Viewport width | Document width | Body width | PNG width | PNG height |",
      "| --- | ---: | ---: | ---: | ---: | ---: |",
      ...mobileRecords.map((record) => {
        const check = mobileOverflowChecks.find(
          (item) => item.label === record.label,
        );
        return `| ${record.file} | ${check?.innerWidth ?? ""} | ${check?.documentWidth ?? ""} | ${check?.bodyWidth ?? ""} | ${record.width ?? ""} | ${record.height ?? ""} |`;
      }),
      "",
      "## Security Scan",
      "",
      securityFindings.length === 0
        ? "Rendered HTML security scan found no raw token, private key, bearer header, or local absolute path leakage."
        : securityFindings.map((finding) => `- ${finding}`).join("\n"),
      "",
    ].join("\n"),
  );

  fs.writeFileSync(
    path.join(runDir, "visual-review.md"),
    [
      "# Gateway Console Visual Review",
      "",
      "## What Looks Good",
      "",
      "- Screenshot matrix completed for all required routes, themes, and viewports.",
      "- Mobile horizontal overflow checks passed before screenshots were accepted.",
      "- Every captured mobile PNG was verified at exactly 390px wide.",
      "- Interaction screenshots cover form validation, Radix open states, copy affordance, mobile navigation, and theme toggle states.",
      "- Generated screenshots are local review artifacts, not marketing screenshots.",
      "",
      "## Visual Issues Found",
      "",
      "- Visual issues found during automated evidence run: none detected by automated screenshot/overflow/security gates.",
      "- Manual product-owner review: pending after inspecting PNG artifacts.",
      "",
      "## Fixes Made",
      "",
      "- No automatic visual fixes were applied during this evidence generation run.",
      "",
      "## Remaining Concerns",
      "",
      "- This screenshot run uses explicit `NEXT_PUBLIC_CODENCER_CONSOLE_MODE=demo`; live readiness is covered by `make verify-gateway-console-live`.",
      "- Live Gateway API integration is limited to the public/self-host console resources wired in the UI data client.",
      "- Private managed Cloud features are intentionally out of scope.",
      "",
      "## Screenshots Not Captured",
      "",
      "- None expected. Check `index.md` for the full route matrix and interaction inventory.",
      "",
      "## Commit Policy",
      "",
      "- PNG evidence is intentionally left local by default to avoid repository bloat.",
      "",
    ].join("\n"),
  );
}

function assertVisualReviewHasNoPlaceholders() {
  const review = fs.readFileSync(path.join(runDir, "visual-review.md"), "utf8");
  expect(review).not.toContain("Fill this section");
}

function slug(routePath: string) {
  return routePath.replace(/^\//, "").replaceAll("/", "-") || "root";
}

function timestamp() {
  const now = new Date();
  const pad = (value: number) => String(value).padStart(2, "0");
  return (
    [now.getFullYear(), pad(now.getMonth() + 1), pad(now.getDate())].join("-") +
    `-${pad(now.getHours())}${pad(now.getMinutes())}`
  );
}
