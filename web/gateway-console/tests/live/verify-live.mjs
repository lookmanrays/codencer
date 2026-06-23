import { chromium, expect } from "@playwright/test";
import crypto from "node:crypto";
import fs from "node:fs/promises";
import http from "node:http";
import os from "node:os";
import path from "node:path";
import { execFile, spawn } from "node:child_process";
import { promisify } from "node:util";

const repoRoot = path.resolve(process.cwd(), "../..");
const binaryDir =
  process.env.CODENCER_E2E_BIN_DIR ?? path.join(repoRoot, "bin");
const codencerBinary = path.join(binaryDir, "codencer");
const connectorBinary = path.join(binaryDir, "codencer-connectord");
const daemonBinary = path.join(binaryDir, "orchestratord");
const gatewayBinary = path.join(binaryDir, "codencer-gatewayd");
const relayBinary = path.join(binaryDir, "codencer-relayd");
const gatewayToken = "gateway-live-secret";
const relayToken = "relay-live-secret";
const operatorCode = "operator-code";
const execFileAsync = promisify(execFile);
const executorAdapter = process.env.CODENCER_E2E_EXECUTOR_ADAPTER ?? "fake";
const executorProfile =
  process.env.CODENCER_E2E_EXECUTOR_PROFILE ??
  (executorAdapter === "fake" ? "fake-success" : executorAdapter);
const realExecutorGate = executorAdapter !== "fake";
if (realExecutorGate) {
  assertNoRealGateSimulationEnv();
}
if (executorAdapter === "codex" && executorProfile !== "codex-workspace") {
  throw new Error(
    `real Codex gate requires profile=codex-workspace, got ${executorProfile}`,
  );
}
const taskTimeoutSeconds = parsePositiveInt(
  process.env.CODENCER_E2E_EXECUTOR_TIMEOUT_SECONDS,
  executorAdapter === "fake" ? 120 : 300,
  "CODENCER_E2E_EXECUTOR_TIMEOUT_SECONDS",
);
const relayProxyTimeoutSeconds = taskTimeoutSeconds + 30;
const gatewayRelayRequestTimeoutSeconds = relayProxyTimeoutSeconds + 30;
const uiSubmitTimeoutMs = (gatewayRelayRequestTimeoutSeconds + 30) * 1000;
const executorGoal =
  executorAdapter === "fake"
    ? "Run fake-safe task from live Gateway Console."
    : "Inspect the project README and return a short summary. Do not modify files.";

const tmpRoot = await fs.mkdtemp(
  path.join(os.tmpdir(), "codencer-console-live-"),
);
const processes = [];
const servers = [];

try {
  const codencerHome = path.join(tmpRoot, "codencer-home");
  await fs.mkdir(codencerHome, { recursive: true });
  const stack = await startLocalSelfHostStack(tmpRoot);
  const gatewayPort = await freePort();
  const consolePort = await freePort();
  const gatewayBase = `http://127.0.0.1:${gatewayPort}`;
  const configPath = path.join(tmpRoot, "gateway-config.json");
  await fs.writeFile(
    configPath,
    JSON.stringify(
      {
        version: 1,
        public_base_url: gatewayBase,
        mcp_url: `${gatewayBase}/mcp`,
        listen_addr: `127.0.0.1:${gatewayPort}`,
        relay_request_timeout_seconds: gatewayRelayRequestTimeoutSeconds,
        store: { path: path.join(tmpRoot, "gateway.db") },
        default_relay: {
          url: stack.relayUrl,
          token_env: "CODENCER_LIVE_RELAY_TOKEN",
        },
        auth: {
          mode: "bearer-dev",
          token_env: "CODENCER_LIVE_GATEWAY_TOKEN",
        },
        oauth_dev: {
          enabled: true,
          issuer: gatewayBase,
          client_id: "codencer-chatgpt-dev",
          operator_code_hash: sha256Hex(operatorCode),
          scopes: [
            "projects:read",
            "projects:write",
            "runs:read",
            "runs:write",
          ],
        },
      },
      null,
      2,
    ),
  );

  const gateway = spawnProcess(
    gatewayBinary,
    ["serve", "--config", configPath],
    {
      CODENCER_HOME: codencerHome,
      CODENCER_LIVE_GATEWAY_TOKEN: gatewayToken,
      CODENCER_LIVE_RELAY_TOKEN: relayToken,
    },
  );
  await waitForJSON(`${gatewayBase}/health`);

  await assertGatewayCollectionEndpoints(
    gatewayBase,
    gatewayToken,
    "direct Gateway empty collections",
    { expectLiveMetadata: false },
  );

  spawnProcess(
    "npm",
    [
      "run",
      "start",
      "--",
      "--hostname",
      "127.0.0.1",
      "--port",
      String(consolePort),
    ],
    {
      CODENCER_HOME: codencerHome,
      CODENCER_GATEWAY_API_BASE: gatewayBase,
      CODENCER_GATEWAY_MCP_TOKEN: gatewayToken,
      NEXT_PUBLIC_CODENCER_CONSOLE_MODE: "live",
    },
  );
  const consoleBase = `http://127.0.0.1:${consolePort}`;
  await waitForHTML(consoleBase);
  await assertGatewayCollectionEndpoints(
    consoleBase,
    "",
    "Console proxy empty collections",
    { expectLiveMetadata: false },
  );

  const browser = await chromium.launch();
  try {
    const page = await browser.newPage({
      viewport: { width: 1280, height: 900 },
    });

    await page.goto(`${consoleBase}/console`);
    await expect(
      page.getByRole("heading", { name: /self-host bridge status/i }),
    ).toBeVisible();
    await expect(
      page.getByText(/Gateway Console live data unavailable/i),
    ).toBeHidden();
    await assertNoDemoOrSecretLeak(page);

    await startConnectorThroughGateway(gatewayBase, stack);
    await assertGatewayCollectionEndpoints(
      gatewayBase,
      gatewayToken,
      "direct Gateway live metadata",
      { expectLiveMetadata: true },
    );
    await assertGatewayCollectionEndpoints(
      consoleBase,
      "",
      "Console proxy live metadata",
      { expectLiveMetadata: true },
    );
    const mcpProof = await runGatewayMCPProof(gatewayBase, gatewayToken);
    console.log(`gateway-console-live: mcp_run=${mcpProof.runId}`);
    if (realExecutorGate) {
      assertNoSimulationText(stack.daemonProcess.__codencerLog, "daemon log");
    }

    await page.goto(`${consoleBase}/console`);
    await expect(
      page.getByRole("heading", { name: /self-host bridge status/i }),
    ).toBeVisible();
    await expect(page.getByText("gateway-dev@codencer.local")).toBeVisible();
    await expect(
      page.getByText(/Gateway Console live data unavailable/i),
    ).toBeHidden();
    await assertNoDemoOrSecretLeak(page);

    await page.goto(`${consoleBase}/console/relays`);
    await expect(page.getByText("Default Codencer Relay")).toBeVisible();
    await page.getByLabel(/profile name/i).fill("test-self-host");
    await page.getByLabel(/relay url/i).fill(stack.relayUrl);
    await page
      .getByLabel(/token environment variable/i)
      .fill("CODENCER_LIVE_RELAY_TOKEN");
    await page.getByRole("button", { name: /save relay profile/i }).click();
    await expect(
      page.getByRole("heading", { name: "test-self-host" }),
    ).toBeVisible();
    await page.reload();
    await expect(
      page.getByRole("heading", { name: "test-self-host" }),
    ).toBeVisible();
    await page
      .getByRole("button", { name: /^remove$/i })
      .last()
      .click();
    await page
      .getByRole("button", { name: /^remove$/i })
      .last()
      .click();
    await expect(
      page.getByRole("heading", { name: "test-self-host" }),
    ).toBeHidden();
    await page.reload();
    await expect(
      page.getByRole("heading", { name: "test-self-host" }),
    ).toBeHidden();
    await assertNoDemoOrSecretLeak(page);

    await page.goto(`${consoleBase}/console/connectors`);
    await expect(page.getByRole("cell", { name: "live-host" })).toBeVisible();
    await expect(
      page.getByRole("cell", { name: /darwin\/arm64|linux\/amd64/ }),
    ).toBeVisible();
    await assertNoDemoOrSecretLeak(page);

    await page.goto(`${consoleBase}/console/projects`);
    await expect(
      page.getByRole("cell", { name: "Codencer", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("row", {
        name: /Codencer default live-host repo · [a-f0-9]{16} online none/i,
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("combobox", { name: "Executor" }),
    ).toContainText(executorProfile);
    if (executorProfile === "codex-workspace") {
      await expect(page.getByLabel(/title/i)).toHaveValue(
        "Codex workspace smoke task",
      );
      await expect(page.getByLabel(/goal/i)).toHaveValue(executorGoal);
      await expect(page.getByLabel(/title/i)).not.toHaveValue(
        "Gateway Console fake-safe task",
      );
      await expect(page.getByLabel(/timeout seconds/i)).toHaveValue("300");
    }
    await expect(page.getByText("Route preview")).toBeVisible();
    await expect(page.getByText(executorProfile).first()).toBeVisible();
    await page.getByLabel(/goal/i).fill(executorGoal);
    await page.getByLabel(/timeout seconds/i).fill(String(taskTimeoutSeconds));
    await page.getByRole("button", { name: /^submit$/i }).click();
    await expect(page.getByText(/^Result$/i)).toBeVisible({
      timeout: uiSubmitTimeoutMs,
    });
    if (realExecutorGate) {
      await expect(page.getByText("Real executor").first()).toBeVisible({
        timeout: uiSubmitTimeoutMs,
      });
    }
    await expect(page.getByText(/Run ID/i)).toBeVisible({
      timeout: uiSubmitTimeoutMs,
    });
    await expect(page.getByText(/run-/i).first()).toBeVisible({
      timeout: uiSubmitTimeoutMs,
    });
    await expect(page.getByText(/completed/i).first()).toBeVisible({
      timeout: uiSubmitTimeoutMs,
    });
    await expect(page.getByText(executorProfile).first()).toBeVisible({
      timeout: uiSubmitTimeoutMs,
    });
    await expect(page.getByText(/Summary/i)).toBeVisible({
      timeout: uiSubmitTimeoutMs,
    });
    await expect(
      page.getByText(/summary=|codencer|readme|artifact|completed/i).first(),
    ).toBeVisible({
      timeout: uiSubmitTimeoutMs,
    });
    const fullRunLink = page.getByRole("link", { name: /view full run/i });
    await expect(fullRunLink).toBeVisible();
    await fullRunLink.click();
    await expect(page.getByRole("heading", { name: /full run/i })).toBeVisible({
      timeout: uiSubmitTimeoutMs,
    });
    await expect(page.getByText(executorProfile).first()).toBeVisible();
    await expect(page.getByText(/Run ID/i).first()).toBeVisible();
    await expect(page.getByText(/Result/i).first()).toBeVisible();
    if (realExecutorGate) {
      await expect(page.getByText("Real executor").first()).toBeVisible();
      const runsPayload = await getJSON(
        `${gatewayBase}/api/gateway/v1/runs`,
        gatewayToken,
      );
      const uiRun = (runsPayload.runs ?? []).find(
        (run) =>
          run.executor_profile === executorProfile &&
          run.title === "Codex workspace smoke task",
      );
      if (!uiRun) {
        throw new Error(
          `real executor UI run was not recorded in run history: ${JSON.stringify(
            runsPayload,
          )}`,
        );
      }
      assertRealRunHistoryRecord("Gateway Console UI run history", uiRun);
      assertNoSimulationText(stack.daemonProcess.__codencerLog, "daemon log");
    }
    await expect(page.getByText(/event timeline/i)).toBeVisible();
    await expect(page.getByText(/task_submitted/i).first()).toBeVisible();
    await expect(page.getByText(/run_completed/i).first()).toBeVisible();
    await assertNoDemoOrSecretLeak(page);

    await page.goto(`${consoleBase}/console/runs`);
    await expect(
      page.getByRole("heading", { name: /run history/i }),
    ).toBeVisible();
    await expect(
      page
        .getByText(
          /Codex workspace smoke task|Gateway Console fake-safe task|Gateway Console task/i,
        )
        .first(),
    ).toBeVisible();
    await expect(page.getByText(executorProfile).first()).toBeVisible();
    await expect(page.getByText(/Run ID/i).first()).toBeVisible();
    await expect(page.getByText(/Result/i).first()).toBeVisible();
    if (realExecutorGate) {
      await expect(page.getByText("Real executor").first()).toBeVisible();
    }
    await expect(
      page.getByRole("link", { name: /view details/i }).first(),
    ).toBeVisible();
    await assertNoDemoOrSecretLeak(page);

    await page.goto(`${consoleBase}/console/audit`);
    await expect(
      page.getByRole("link", { name: /task_submitted/i }).first(),
    ).toBeVisible();
    await page
      .getByRole("link", { name: /task_submitted/i })
      .first()
      .click();
    await expect(
      page.getByRole("heading", { name: /full run/i }),
    ).toBeVisible();
    await assertNoDemoOrSecretLeak(page);

    await page.goto(`${consoleBase}/console/activation`);
    await expect(
      page.getByRole("heading", { name: /gateway-first setup/i }),
    ).toBeVisible();
    await expect(
      page.getByText(`codencer login --gateway ${gatewayBase}`),
    ).toBeVisible();
    await assertNoDemoOrSecretLeak(page);

    await page.goto(`${consoleBase}/console/audit`);
    await expect(page.getByText(/connector\.login/i).first()).toBeVisible();
    await expect(page.getByText(/task_submitted/i).first()).toBeVisible();
    await expect(page.getByText(/route_resolved/i).first()).toBeVisible();
    await expect(page.getByText(/relay_selected/i).first()).toBeVisible();
    await expect(page.getByText(/connector_selected/i).first()).toBeVisible();
    await expect(page.getByText(/executor_selected/i).first()).toBeVisible();
    await expect(page.getByText(/run_started/i).first()).toBeVisible();
    await expect(page.getByText(/run_completed/i).first()).toBeVisible();
    await expect(page.getByText(/report_read/i).first()).toBeVisible();
    await expect(page.getByText("relay.add").first()).toBeVisible();
    await expect(page.getByText("relay.remove").first()).toBeVisible();
    await assertNoDemoOrSecretLeak(page);

    await page.goto(`${consoleBase}/console/settings`);
    await expect(
      page.getByRole("heading", { name: /console settings/i }),
    ).toBeVisible();
    await expect(
      page.getByText("Personal Gateway Workspace").first(),
    ).toBeVisible();
    await expect(page.getByText(`${gatewayBase}/mcp`)).toBeVisible();
    await assertNoDemoOrSecretLeak(page);

    const device = await postJSON(
      `${gatewayBase}/api/gateway/v1/device/authorize`,
      {
        email: "browser@example.com",
        display_name: "Browser Operator",
      },
    );
    await page.goto(`${consoleBase}/device`);
    await page.getByLabel(/user code/i).fill("BAD-CODE");
    await page.getByRole("button", { name: /approve device/i }).click();
    await expect(page.getByText(/device approval failed/i)).toBeVisible();
    await page.getByLabel(/user code/i).fill(device.user_code);
    await page.getByRole("button", { name: /approve device/i }).click();
    await expect(page.getByText(/device login approved/i)).toBeVisible();
    await assertNoDemoOrSecretLeak(page);

    const verifier = "live-oauth-code-verifier";
    const oauthURL = new URL(`${consoleBase}/oauth/authorize`);
    oauthURL.searchParams.set("response_type", "code");
    oauthURL.searchParams.set("client_id", "codencer-chatgpt-dev");
    oauthURL.searchParams.set("redirect_uri", "http://127.0.0.1/callback");
    oauthURL.searchParams.set("scope", "projects:read projects:write");
    oauthURL.searchParams.set("state", "state-live");
    oauthURL.searchParams.set("code_challenge", codeChallengeS256(verifier));
    oauthURL.searchParams.set("code_challenge_method", "S256");
    oauthURL.searchParams.set("resource", `${gatewayBase}/mcp`);
    await page.goto(oauthURL.toString());
    await expect(page.getByText("Codencer MCP client")).toBeVisible();
    await page.getByLabel(/operator approval code/i).fill(operatorCode);
    await page.getByRole("button", { name: /^approve$/i }).click();
    await expect(page.getByText(/consent approved/i)).toBeVisible();
    await assertNoDemoOrSecretLeak(page);

    signalProcess(gateway, "SIGTERM");
    await waitForGatewayDown(gatewayBase);
    const brokenContext = await browser.newContext({
      viewport: { width: 1280, height: 900 },
    });
    try {
      const brokenPage = await brokenContext.newPage();
      await brokenPage.goto(`${consoleBase}/console`);
      await expect(
        brokenPage.getByText(/Gateway Console live data unavailable/i),
      ).toBeVisible();
      await assertNoDemoOrSecretLeak(brokenPage);
    } finally {
      await brokenContext.close();
    }
  } finally {
    await browser.close();
  }

  console.log(
    `gateway-console-live: ok gateway=${gatewayBase} console=${consoleBase} relay=${stack.relayUrl}`,
  );
} finally {
  for (const child of processes.reverse()) await stopProcess(child);
  for (const server of servers.reverse())
    await new Promise((resolve) => server.close(resolve));
  await fs.rm(tmpRoot, { force: true, recursive: true });
}

async function runGatewayMCPProof(gatewayBase, token) {
  let sessionId = "";
  async function mcpPost(payload) {
    const response = await fetch(`${gatewayBase}/mcp`, {
      body: JSON.stringify(payload),
      headers: {
        Accept: "application/json, text/event-stream",
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
        "MCP-Protocol-Version": "2025-11-25",
        ...(sessionId ? { "MCP-Session-Id": sessionId } : {}),
      },
      method: "POST",
    });
    const returnedSession = response.headers.get("MCP-Session-Id");
    if (returnedSession) sessionId = returnedSession;
    const text = await response.text();
    if (!response.ok) {
      throw new Error(`Gateway MCP returned ${response.status}: ${text}`);
    }
    assertNoSensitiveEndpointLeak(text, "Gateway MCP");
    return JSON.parse(text);
  }
  async function tool(name, args) {
    const payload = await mcpPost({
      jsonrpc: "2.0",
      id: name,
      method: "tools/call",
      params: { name, arguments: args },
    });
    const result = payload.result ?? {};
    if (result.isError) {
      throw new Error(
        `${name} returned MCP tool error: ${JSON.stringify(payload)}`,
      );
    }
    return result.structuredContent ?? {};
  }

  const initialized = await mcpPost({
    jsonrpc: "2.0",
    id: "init",
    method: "initialize",
    params: {
      clientInfo: { name: "gateway-console-live", version: "v0" },
      protocolVersion: "2025-11-25",
    },
  });
  if (!sessionId || !initialized.result?.serverInfo) {
    throw new Error(`MCP initialize failed: ${JSON.stringify(initialized)}`);
  }
  const tools = await mcpPost({
    jsonrpc: "2.0",
    id: "tools",
    method: "tools/list",
    params: {},
  });
  const toolNames = JSON.stringify(tools.result?.tools ?? []);
  for (const expected of [
    "codencer.list_projects",
    "codencer.submit_project_task_and_wait",
    "codencer.get_run_report",
  ]) {
    if (!toolNames.includes(expected)) {
      throw new Error(`MCP tools/list missing ${expected}: ${toolNames}`);
    }
  }

  const projects = await tool("codencer.list_projects", {});
  const project = (projects.projects ?? []).find(
    (item) => item.project_id === "codencer",
  );
  const relay = project?.relay_profiles?.[0];
  const location = relay?.locations?.find((item) => item.online) ?? {};
  if (!project || !relay || !location.machine_id) {
    throw new Error(
      `MCP list_projects missing live location: ${JSON.stringify(projects)}`,
    );
  }

  const submit = await tool("codencer.submit_project_task_and_wait", {
    goal:
      executorAdapter === "fake"
        ? "Run fake-safe task through Gateway MCP."
        : `Run a safe deterministic task through Gateway MCP with ${executorProfile}.`,
    machine_id: location.machine_id,
    profile: executorProfile,
    project_id: "codencer",
    relay_profile_id: relay.relay_profile_id,
    timeout_seconds: taskTimeoutSeconds,
    title: "Gateway MCP live task",
  });
  const runId =
    submit.run_id ??
    submit.run?.id ??
    submit.task?.run_id ??
    submit.task?.runId;
  if (!runId || submit.status !== "completed") {
    throw new Error(
      `MCP submit did not complete with run_id: ${JSON.stringify(submit)}`,
    );
  }

  const report = await tool("codencer.get_run_report", {
    machine_id: location.machine_id,
    project_id: "codencer",
    relay_profile_id: relay.relay_profile_id,
    run_id: runId,
  });
  if (realExecutorGate) {
    assertRealExecutorReport("Gateway MCP run report", report);
  }
  const reportRunId = report.run_id ?? report.run?.id;
  if (reportRunId !== runId || report.status !== "completed") {
    throw new Error(`MCP get_run_report failed: ${JSON.stringify(report)}`);
  }

  const audit = await getJSON(
    `${gatewayBase}/api/gateway/v1/audit-events`,
    token,
  );
  const eventTypes = (audit.events ?? []).map((event) => event.type);
  for (const expected of [
    "task_submitted",
    "route_resolved",
    "relay_selected",
    "connector_selected",
    "executor_selected",
    "run_started",
    "run_completed",
    "report_read",
  ]) {
    if (!eventTypes.includes(expected)) {
      throw new Error(
        `Gateway audit missing ${expected} after MCP proof: ${JSON.stringify(audit)}`,
      );
    }
  }
  assertNoSensitiveEndpointLeak(JSON.stringify(audit), "Gateway audit");
  return { runId };
}

async function startLocalSelfHostStack(root) {
  const repo = path.join(root, "repo");
  const state = path.join(root, "daemon-state");
  const connectorHome = path.join(root, "connector-home");
  const connectorConfig = path.join(root, "connector.json");
  await fs.mkdir(repo, { recursive: true });
  await fs.writeFile(
    path.join(repo, "README.md"),
    "console live verification\n",
  );
  await runCommand("git", ["-C", repo, "init", "-q"]);
  await runCommand("git", ["-C", repo, "add", "README.md"]);
  await runCommand("git", [
    "-C",
    repo,
    "-c",
    "user.name=Codencer",
    "-c",
    "user.email=codencer@example.invalid",
    "commit",
    "-q",
    "-m",
    "initial",
  ]);

  const daemonPort = await freePort();
  const relayPort = await freePort();
  const daemonUrl = `http://127.0.0.1:${daemonPort}`;
  const relayUrl = `http://127.0.0.1:${relayPort}`;
  const daemonConfig = path.join(root, "daemon.json");
  await fs.writeFile(
    daemonConfig,
    JSON.stringify(
      {
        log_level: "error",
        db_path: path.join(state, "codencer.db"),
        artifact_root: path.join(state, "artifacts"),
        workspace_root: path.join(state, "workspace"),
        repo_root: repo,
        host: "127.0.0.1",
        port: daemonPort,
      },
      null,
      2,
    ),
  );
  const daemonEnv = {
    ALL_ADAPTERS_SIMULATION_MODE: "0",
    ARTIFACT_ROOT: path.join(state, "artifacts"),
    CLAUDE_SIMULATION_MODE: "0",
    CODEX_SIMULATION_MODE: "0",
    DB_PATH: path.join(state, "codencer.db"),
    HOST: "127.0.0.1",
    LOG_LEVEL: realExecutorGate ? "info" : "error",
    PORT: String(daemonPort),
    REPO_ROOT: repo,
    WORKSPACE_ROOT: path.join(state, "workspace"),
  };
  if (executorAdapter === "codex") {
    daemonEnv.CODEX_BINARY =
      process.env.CODEX_BINARY ??
      process.env.CODENCER_E2E_REAL_EXECUTOR_COMMAND ??
      "codex";
  }
  if (executorAdapter === "claude") {
    daemonEnv.CLAUDE_BINARY =
      process.env.CLAUDE_BINARY ??
      process.env.CODENCER_E2E_REAL_EXECUTOR_COMMAND ??
      "claude";
  }
  const daemonProcess = spawnProcess(
    daemonBinary,
    ["--config", daemonConfig, "--repo-root", repo],
    daemonEnv,
  );
  await waitForJSON(`${daemonUrl}/health`);

  const relayConfig = path.join(root, "relay.json");
  await fs.writeFile(
    relayConfig,
    JSON.stringify(
      {
        host: "127.0.0.1",
        port: relayPort,
        db_path: path.join(root, "relay.db"),
        planner_tokens: [
          { name: "operator", token: relayToken, scopes: ["*"] },
        ],
        proxy_timeout_seconds: relayProxyTimeoutSeconds,
        public_base_url: relayUrl,
      },
      null,
      2,
    ),
  );
  spawnProcess(relayBinary, ["--config", relayConfig], {});
  await waitFor(async () => {
    const response = await fetch(`${relayUrl}/api/v2/status`, {
      headers: { Authorization: `Bearer ${relayToken}` },
    });
    return response.ok;
  }, `waiting for ${relayUrl}/api/v2/status`);

  await runCommand(codencerBinary, ["init", "--json"], {
    CODENCER_HOME: connectorHome,
  });
  await runCommand(
    codencerBinary,
    ["machine", "set-label", "live-host", "--json"],
    {
      CODENCER_HOME: connectorHome,
    },
  );
  await runCommand(
    codencerBinary,
    [
      "project",
      "init",
      "--id",
      "codencer",
      "--repo",
      repo,
      "--adapter",
      executorAdapter,
      "--profile",
      executorProfile,
      "--daemon-url",
      daemonUrl,
      "--share-to-relay",
      "--json",
    ],
    { CODENCER_HOME: connectorHome },
  );

  return {
    connectorConfig,
    connectorHome,
    daemonProcess,
    daemonUrl,
    relayUrl,
    repo,
  };
}

async function startConnectorThroughGateway(gatewayBase, stack) {
  const connectorLogin = await gatewayFetch(gatewayBase, "/connectors/login", {
    relay: "default",
    machine: {
      machine_id: "mach-live",
      hostname: "live-host.local",
      host_label: "live-host",
      os: process.platform === "darwin" ? "darwin" : "linux",
      arch: process.arch === "arm64" ? "arm64" : "amd64",
    },
    label: "live connector",
  });
  await runCommand(
    connectorBinary,
    [
      "enroll",
      "--relay-url",
      connectorLogin.relay_url,
      "--daemon-url",
      stack.daemonUrl,
      "--enrollment-token",
      connectorLogin.enrollment_token,
      "--config",
      stack.connectorConfig,
      "--codencer-home",
      stack.connectorHome,
      "--label",
      "live-host",
    ],
    {},
  );
  const connectorConfig = JSON.parse(
    await fs.readFile(stack.connectorConfig, "utf8"),
  );
  await gatewayFetch(gatewayBase, "/connectors/complete", {
    binding_id: connectorLogin.binding_id,
    relay_connector_id: connectorConfig.connector_id,
    relay_machine_id: connectorConfig.machine_id,
    public_key: connectorConfig.public_key,
  });
  spawnProcess(connectorBinary, ["run", "--config", stack.connectorConfig], {});
  await waitFor(async () => {
    const response = await fetch(`${gatewayBase}/api/gateway/v1/projects`, {
      headers: { Authorization: `Bearer ${gatewayToken}` },
    });
    if (!response.ok) return false;
    const payload = await response.json();
    return JSON.stringify(payload).includes('"project_id":"codencer"');
  }, "waiting for Gateway to list connector project");
}

function spawnProcess(command, args, env) {
  const detached = process.platform !== "win32";
  const child = spawn(command, args, {
    cwd: process.cwd(),
    detached,
    env: { ...process.env, ...env },
    stdio: ["ignore", "pipe", "pipe"],
  });
  child.__codencerLog = "";
  const capture = (stream, data) => {
    const text = data.toString();
    child.__codencerLog += text;
    if (child.__codencerLog.length > 1024 * 1024) {
      child.__codencerLog = child.__codencerLog.slice(-512 * 1024);
    }
    stream.write(`[live] ${data}`);
  };
  child.stdout.on("data", (data) => capture(process.stdout, data));
  child.stderr.on("data", (data) => capture(process.stderr, data));
  processes.push(child);
  return child;
}

async function stopProcess(child) {
  if (child.exitCode !== null || child.signalCode !== null) return;
  await new Promise((resolve) => {
    const timeout = setTimeout(() => {
      signalProcess(child, "SIGKILL");
      resolve();
    }, 5_000);
    child.once("close", () => {
      clearTimeout(timeout);
      resolve();
    });
    signalProcess(child, "SIGTERM");
  });
}

function signalProcess(child, signal) {
  if (child.exitCode !== null || child.signalCode !== null) return;
  if (process.platform !== "win32") {
    try {
      process.kill(-child.pid, signal);
      return;
    } catch (error) {
      if (error.code !== "ESRCH") throw error;
    }
  }
  child.kill(signal);
}

async function runCommand(command, args, env = {}) {
  try {
    return await execFileAsync(command, args, {
      cwd: repoRoot,
      env: { ...process.env, ...env },
      maxBuffer: 4 * 1024 * 1024,
    });
  } catch (error) {
    const stdout = error.stdout ? `\nstdout:\n${error.stdout}` : "";
    const stderr = error.stderr ? `\nstderr:\n${error.stderr}` : "";
    throw new Error(
      `${command} ${args.join(" ")} failed: ${error.message}${stdout}${stderr}`,
    );
  }
}

async function gatewayFetch(base, path, body) {
  return postJSON(`${base}/api/gateway/v1${path}`, body, gatewayToken);
}

function assertNoRealGateSimulationEnv() {
  const names = Array.from(
    new Set(
      [
        "ALL_ADAPTERS_SIMULATION_MODE",
        `${executorAdapter.toUpperCase().replaceAll("-", "_")}_SIMULATION_MODE`,
        executorAdapter === "codex" ? "CODEX_SIMULATION_MODE" : "",
        executorAdapter === "claude" ? "CLAUDE_SIMULATION_MODE" : "",
      ].filter(Boolean),
    ),
  );
  const diagnostics = names.map((name) => {
    const value = process.env[name];
    return `${name}=${value == null ? "<unset>" : value}`;
  });
  console.log(
    `gateway-console-live: real executor simulation env preflight ${diagnostics.join(" ")}`,
  );
  const bad = names.filter((name) =>
    ["1", "true"].includes(String(process.env[name] ?? "").toLowerCase()),
  );
  if (bad.length > 0) {
    throw new Error(
      `real executor gate refuses simulation environment values: ${diagnostics.join(" ")}`,
    );
  }
}

function assertRealRunHistoryRecord(label, record) {
  assertNoSimulationText(JSON.stringify(record), label);
  if (record.executor_profile !== executorProfile) {
    throw new Error(
      `${label} expected executor_profile=${executorProfile}, got ${record.executor_profile}`,
    );
  }
  if (!record.report || Object.keys(record.report).length === 0) {
    throw new Error(`${label} missing sanitized report payload`);
  }
  assertRealExecutorReport(`${label} report`, record.report);
  if (!hasRealOutputOrArtifact(record)) {
    throw new Error(
      `${label} did not include real executor output or artifact evidence: ${JSON.stringify(
        record,
      )}`,
    );
  }
}

function assertRealExecutorReport(label, payload) {
  const serialized = JSON.stringify(payload);
  assertNoSimulationText(serialized, label);
  const simulationValues = findValuesByKey(payload, "is_simulation");
  if (simulationValues.some((value) => value === true)) {
    throw new Error(`${label} reported is_simulation=true: ${serialized}`);
  }
  if (!simulationValues.some((value) => value === false)) {
    throw new Error(
      `${label} did not report is_simulation=false: ${serialized}`,
    );
  }
  const adapterValues = findStringValuesByKey(payload, "adapter");
  if (!adapterValues.includes(executorAdapter)) {
    throw new Error(
      `${label} expected adapter=${executorAdapter}, got ${JSON.stringify(
        adapterValues,
      )}: ${serialized}`,
    );
  }
  const profileValues = [
    ...findStringValuesByKey(payload, "profile"),
    ...findStringValuesByKey(payload, "executor_profile"),
  ];
  if (!profileValues.includes(executorProfile)) {
    throw new Error(
      `${label} expected profile=${executorProfile}, got ${JSON.stringify(
        profileValues,
      )}: ${serialized}`,
    );
  }
  if (!hasRealOutputOrArtifact(payload)) {
    throw new Error(
      `${label} did not include real executor output or artifact evidence: ${serialized}`,
    );
  }
}

function assertNoSimulationText(text, label) {
  if (!text) return;
  for (const pattern of [
    /Simulation Mode/i,
    /Executing Simulated codex/i,
    /Simulated successful codex task/i,
  ]) {
    if (pattern.test(text)) {
      throw new Error(`${label} contained simulated executor text: ${pattern}`);
    }
  }
}

function hasRealOutputOrArtifact(payload) {
  const outputKeys = new Set([
    "details",
    "message",
    "raw_output",
    "result_details",
    "result_summary",
    "stderr",
    "stdout",
    "summary",
    "text",
  ]);
  const outputs = findStringEntries(payload)
    .filter(({ key, value }) => outputKeys.has(key) && value.trim().length > 0)
    .map(({ value }) => value.trim())
    .filter((value) => !/^(completed|ok|success)$/i.test(value));
  const artifactNames = findStringValuesByKey(payload, "name").filter((value) =>
    /codex|stdout|stderr|last[-_ ]?message|result|report/i.test(value),
  );
  return outputs.length > 0 || artifactNames.length > 0;
}

function findStringValuesByKey(value, key) {
  return findValuesByKey(value, key).filter((item) => typeof item === "string");
}

function findValuesByKey(value, key, seen = new WeakSet()) {
  if (!value || typeof value !== "object") return [];
  if (seen.has(value)) return [];
  seen.add(value);
  const out = [];
  if (Array.isArray(value)) {
    for (const item of value) out.push(...findValuesByKey(item, key, seen));
    return out;
  }
  for (const [entryKey, entryValue] of Object.entries(value)) {
    if (entryKey === key) out.push(entryValue);
    out.push(...findValuesByKey(entryValue, key, seen));
  }
  return out;
}

function findStringEntries(value, seen = new WeakSet()) {
  if (!value || typeof value !== "object") return [];
  if (seen.has(value)) return [];
  seen.add(value);
  const out = [];
  if (Array.isArray(value)) {
    for (const item of value) out.push(...findStringEntries(item, seen));
    return out;
  }
  for (const [key, entryValue] of Object.entries(value)) {
    if (typeof entryValue === "string") out.push({ key, value: entryValue });
    out.push(...findStringEntries(entryValue, seen));
  }
  return out;
}

async function assertGatewayCollectionEndpoints(
  base,
  token,
  label,
  { expectLiveMetadata },
) {
  const machines = await getJSON(`${base}/api/gateway/v1/machines`, token);
  const connectors = await getJSON(`${base}/api/gateway/v1/connectors`, token);
  const executors = await getJSON(`${base}/api/gateway/v1/executors`, token);
  const projects = await getJSON(`${base}/api/gateway/v1/projects`, token);
  const runs = await getJSON(`${base}/api/gateway/v1/runs`, token);
  const audit = await getJSON(`${base}/api/gateway/v1/audit-events`, token);
  const activation = await getJSON(
    `${base}/api/gateway/v1/activation/commands`,
    token,
  );

  const machineList = assertArrayField(machines, "machines", label);
  const connectorList = assertArrayField(connectors, "connectors", label);
  const executorList = assertArrayField(executors, "executors", label);
  const projectList = assertArrayField(projects, "projects", label);
  assertArrayField(runs, "runs", label);
  assertArrayField(projects, "relay_errors", label);
  assertArrayField(audit, "audit_events", label);
  assertArrayField(audit, "events", label);
  assertArrayField(activation, "activation_commands", label);
  assertArrayField(activation, "commands", label);

  const serialized = JSON.stringify({
    activation,
    audit,
    connectors,
    executors,
    machines,
    projects,
    runs,
  });
  if (serialized.includes(":null")) {
    throw new Error(`${label} returned a null collection: ${serialized}`);
  }
  assertNoSensitiveEndpointLeak(serialized, label);
  for (const expected of [
    "codex-workspace",
    "codex-full",
    "claude-default",
    "fake-success",
  ]) {
    if (!executorList.some((executor) => executor.id === expected)) {
      throw new Error(
        `${label} executor metadata missing ${expected}: ${JSON.stringify(
          executors,
        )}`,
      );
    }
  }

  if (expectLiveMetadata) {
    if (
      !machineList.some(
        (machine) =>
          machine.host_label === "live-host" &&
          (machine.status === "online" || machine.status === "active"),
      )
    ) {
      throw new Error(
        `${label} did not expose live machine metadata: ${JSON.stringify(
          machines,
        )}`,
      );
    }
    if (
      !connectorList.some(
        (connector) =>
          connector.machine_id &&
          connector.relay_profile_id === "default" &&
          connector.status === "online",
      )
    ) {
      throw new Error(
        `${label} did not expose live connector metadata: ${JSON.stringify(
          connectors,
        )}`,
      );
    }
    if (
      !projectList.some((project) =>
        JSON.stringify(project).includes('"host_label":"live-host"'),
      )
    ) {
      throw new Error(
        `${label} did not expose live project location metadata: ${JSON.stringify(
          projects,
        )}`,
      );
    }
  }
}

function assertArrayField(payload, key, label) {
  if (!Array.isArray(payload?.[key])) {
    throw new Error(
      `${label} expected ${key} to be an array: ${JSON.stringify(payload)}`,
    );
  }
  return payload[key];
}

function assertNoSensitiveEndpointLeak(body, label) {
  for (const forbidden of [
    "/Users/",
    "/tmp/",
    "/var/folders/",
    ".codencer-live-test",
    "relay-live-secret",
    "gateway-live-secret",
    "public_key",
    "daemon_url",
    "planner_token",
    '"repo_root"',
    '"path"',
  ]) {
    if (body.includes(forbidden)) {
      throw new Error(`${label} leaked ${forbidden}: ${body}`);
    }
  }
}

async function getJSON(url, token = "") {
  const response = await fetch(url, {
    headers: {
      Accept: "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
  });
  const text = await response.text();
  if (!response.ok) {
    throw new Error(`${url} returned ${response.status}: ${text}`);
  }
  return JSON.parse(text);
}

async function postJSON(url, body, token = "") {
  const response = await fetch(url, {
    body: JSON.stringify(body),
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    method: "POST",
  });
  const text = await response.text();
  if (!response.ok) {
    throw new Error(`${url} returned ${response.status}: ${text}`);
  }
  return JSON.parse(text);
}

async function waitForJSON(url) {
  await waitFor(async () => {
    const response = await fetch(url);
    return response.ok;
  }, `waiting for ${url}`);
}

async function waitForHTML(url) {
  await waitFor(async () => {
    const response = await fetch(url);
    return response.ok && (await response.text()).includes("<html");
  }, `waiting for ${url}`);
}

async function waitForGatewayDown(base) {
  await waitFor(async () => {
    try {
      const response = await fetch(`${base}/health`);
      return !response.ok;
    } catch {
      return true;
    }
  }, `waiting for ${base} to stop`);
}

async function waitFor(fn, label) {
  const deadline = Date.now() + 60_000;
  let lastError;
  while (Date.now() < deadline) {
    try {
      if (await fn()) return;
    } catch (err) {
      lastError = err;
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  throw new Error(
    `${label} timed out${lastError ? `: ${lastError.message}` : ""}`,
  );
}

async function freePort() {
  const server = http.createServer();
  await listen(server, 0);
  const port = server.address().port;
  await new Promise((resolve) => server.close(resolve));
  return port;
}

function listen(server, port) {
  return new Promise((resolve) => server.listen(port, "127.0.0.1", resolve));
}

async function assertNoDemoOrSecretLeak(page) {
  const html = await page.content();
  for (const forbidden of [
    "Demo Operator",
    "wsl connector",
    "docs-site",
    "Mock data mode",
    "Demo data mode",
    "relay-live-secret",
    "gateway-live-secret",
    "relay-enroll-secret",
    "live-public-key",
    "/Users/",
    "/tmp/",
    "/var/folders/",
    "/home/",
    "report_path",
    "logs_ref",
    "normalized_task_ref",
    "original_input_ref",
  ]) {
    if (html.includes(forbidden)) {
      throw new Error(`live console leaked forbidden content: ${forbidden}`);
    }
  }
}

function sha256Hex(value) {
  return crypto.createHash("sha256").update(value).digest("hex");
}

function codeChallengeS256(verifier) {
  return crypto.createHash("sha256").update(verifier).digest("base64url");
}

function parsePositiveInt(value, fallback, name) {
  if (value == null || value === "") return fallback;
  const parsed = Number.parseInt(value, 10);
  if (!Number.isFinite(parsed) || parsed <= 0 || String(parsed) !== value) {
    throw new Error(`${name} must be a positive integer, got ${value}`);
  }
  return parsed;
}
