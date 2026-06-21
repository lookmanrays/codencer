import { chromium, expect } from "@playwright/test";
import crypto from "node:crypto";
import fs from "node:fs/promises";
import http from "node:http";
import os from "node:os";
import path from "node:path";
import { execFile, spawn } from "node:child_process";
import { promisify } from "node:util";

const repoRoot = path.resolve(process.cwd(), "../..");
const codencerBinary = path.join(repoRoot, "bin", "codencer");
const connectorBinary = path.join(repoRoot, "bin", "codencer-connectord");
const daemonBinary = path.join(repoRoot, "bin", "orchestratord");
const gatewayBinary = path.join(repoRoot, "bin", "codencer-gatewayd");
const relayBinary = path.join(repoRoot, "bin", "codencer-relayd");
const gatewayToken = "gateway-live-secret";
const relayToken = "relay-live-secret";
const operatorCode = "operator-code";
const execFileAsync = promisify(execFile);

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

  await startConnectorThroughGateway(gatewayBase, stack);

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

  const browser = await chromium.launch();
  try {
    const page = await browser.newPage({
      viewport: { width: 1280, height: 900 },
    });

    await page.goto(`${consoleBase}/console`);
    await expect(
      page.getByRole("heading", { name: /self-host bridge status/i }),
    ).toBeVisible();
    await expect(page.getByText("gateway-dev@codencer.local")).toBeVisible();
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
    await page
      .getByLabel(/goal/i)
      .fill("Run fake-safe task from live Gateway Console.");
    await page.getByRole("button", { name: /^submit$/i }).click();
    await expect(page.getByText(/run completed/i)).toBeVisible();
    await expect(page.getByText(/run_id=run-/i)).toBeVisible();
    await expect(page.getByText(/status=completed run_id=run-/i)).toBeVisible();
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
    await expect(page.getByText(/connector\.login/i)).toBeVisible();
    await expect(page.getByText("relay.add")).toBeVisible();
    await expect(page.getByText("relay.remove")).toBeVisible();
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
  spawnProcess(daemonBinary, ["--config", daemonConfig, "--repo-root", repo], {
    ARTIFACT_ROOT: path.join(state, "artifacts"),
    DB_PATH: path.join(state, "codencer.db"),
    HOST: "127.0.0.1",
    LOG_LEVEL: "error",
    PORT: String(daemonPort),
    REPO_ROOT: repo,
    WORKSPACE_ROOT: path.join(state, "workspace"),
  });
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
        proxy_timeout_seconds: 90,
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
      "fake",
      "--profile",
      "fake-success",
      "--daemon-url",
      daemonUrl,
      "--share-to-relay",
      "--json",
    ],
    { CODENCER_HOME: connectorHome },
  );

  return { connectorConfig, connectorHome, daemonUrl, relayUrl, repo };
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
  child.stdout.on("data", (data) => process.stdout.write(`[live] ${data}`));
  child.stderr.on("data", (data) => process.stderr.write(`[live] ${data}`));
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
