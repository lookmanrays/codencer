import type { ConsoleSnapshot } from "@/schemas/console";

export const mockSnapshot: ConsoleSnapshot = {
  user: {
    id: "user_demo",
    email: "operator@example.com",
    displayName: "Demo Operator",
  },
  workspace: {
    id: "ws_personal",
    name: "Personal Gateway Workspace",
    slug: "personal",
    mode: "mock",
  },
  mcpEndpoint: "https://mcp.codencer.dev/mcp",
  relays: [
    {
      id: "default",
      name: "Default managed Relay",
      type: "managed",
      url: "https://relay.codencer.dev",
      tokenRef: "CODENCER_DEFAULT_RELAY_TOKEN",
      enabled: true,
      status: "available",
    },
    {
      id: "personal-vps",
      name: "Personal self-host Relay",
      type: "self_host",
      url: "https://relay.example.com",
      tokenRef: "CODENCER_RELAY_PERSONAL_TOKEN",
      enabled: true,
      status: "available",
    },
    {
      id: "lab",
      name: "Lab Relay",
      type: "self_host",
      url: "https://relay-lab.example.com",
      tokenRef: "CODENCER_RELAY_LAB_TOKEN",
      enabled: false,
      status: "disabled",
    },
  ],
  relayHealth: [
    {
      relayProfileId: "default",
      status: "available",
      latencyMs: 42,
      checkedAt: "2026-06-20T12:15:00Z",
    },
    {
      relayProfileId: "personal-vps",
      status: "available",
      latencyMs: 61,
      checkedAt: "2026-06-20T12:15:00Z",
    },
    {
      relayProfileId: "lab",
      status: "disabled",
      latencyMs: null,
      checkedAt: "2026-06-20T12:15:00Z",
    },
  ],
  machines: [
    {
      id: "mach_mac",
      hostname: "macbook.local",
      hostLabel: "macbook",
      os: "darwin",
      arch: "arm64",
      status: "online",
    },
    {
      id: "mach_wsl",
      hostname: "wsl-coder",
      hostLabel: "wsl2",
      os: "linux",
      arch: "amd64",
      status: "offline",
    },
  ],
  connectors: [
    {
      id: "conn_01",
      machineId: "mach_mac",
      relayProfileId: "default",
      label: "macbook connector",
      status: "online",
      lastSeen: "2026-06-20T12:14:58Z",
    },
    {
      id: "conn_02",
      machineId: "mach_wsl",
      relayProfileId: "personal-vps",
      label: "wsl connector",
      status: "offline",
      lastSeen: "2026-06-19T18:03:20Z",
    },
  ],
  projects: [
    {
      id: "codencer",
      name: "Codencer",
      adapter: "fake",
      profile: "fake-success",
      locations: [
        {
          id: "loc_01",
          projectId: "codencer",
          relayProfileId: "default",
          machineId: "mach_mac",
          hostLabel: "macbook",
          repoLabel: "codencer",
          repoHash: "repo_9f12ac",
          status: "online",
          ambiguity: "none",
        },
        {
          id: "loc_02",
          projectId: "codencer",
          relayProfileId: "personal-vps",
          machineId: "mach_mac",
          hostLabel: "macbook",
          repoLabel: "codencer",
          repoHash: "repo_9f12ac",
          status: "online",
          ambiguity: "relay_profile",
        },
      ],
    },
    {
      id: "docs-site",
      name: "Docs Site",
      adapter: "codex",
      profile: "codex-workspace",
      locations: [
        {
          id: "loc_03",
          projectId: "docs-site",
          relayProfileId: "personal-vps",
          machineId: "mach_wsl",
          hostLabel: "wsl2",
          repoLabel: "docs-site",
          repoHash: "repo_a82bc3",
          status: "offline",
          ambiguity: "machine_location",
        },
      ],
    },
  ],
  auditEvents: [
    {
      id: "evt_01",
      type: "relay.add",
      summary: "Added Personal self-host Relay profile.",
      actor: "operator@example.com",
      createdAt: "2026-06-20T12:10:00Z",
      severity: "info",
    },
    {
      id: "evt_02",
      type: "connector.login",
      summary: "Created connector login binding for macbook.",
      actor: "operator@example.com",
      createdAt: "2026-06-20T12:08:00Z",
      severity: "info",
    },
    {
      id: "evt_03",
      type: "routing.blocker",
      summary: "Project codencer needs relay_profile_id selector.",
      actor: "gateway",
      createdAt: "2026-06-20T12:07:00Z",
      severity: "warning",
    },
  ],
  activationCommands: [
    {
      id: "login",
      title: "Log in to Gateway",
      description:
        "Creates a workspace-bound Gateway session under CODENCER_HOME.",
      target: "gateway",
      command: "codencer login --gateway https://mcp.codencer.dev",
    },
    {
      id: "connector-login",
      title: "Bind local connector",
      description:
        "Requests a short-lived Relay enrollment secret through Gateway; output is redacted.",
      target: "gateway",
      command:
        "codencer connector login --gateway https://mcp.codencer.dev --relay default --json",
    },
    {
      id: "project-init",
      title: "Create project config",
      description:
        "Commits only .codencer/project.json; local state stays in CODENCER_HOME.",
      target: "local",
      command:
        "codencer project init --id codencer --repo . --adapter fake --profile fake-success --json",
    },
    {
      id: "project-share",
      title: "Share project explicitly",
      description: "Connector advertises this project to the selected Relay.",
      target: "local",
      command: "codencer project share codencer --json",
    },
    {
      id: "codex",
      title: "Codex MCP setup",
      description: "AI clients point to Gateway, not a user Relay.",
      target: "client",
      command:
        "codencer setup mcp --client codex --endpoint https://mcp.codencer.dev/mcp --json",
    },
    {
      id: "claude",
      title: "Claude Code MCP setup",
      description: "Generates the Gateway MCP command for Claude Code.",
      target: "client",
      command:
        "codencer setup mcp --client claude-code --endpoint https://mcp.codencer.dev/mcp --json",
    },
    {
      id: "chatgpt",
      title: "ChatGPT custom MCP setup",
      description: "Uses Gateway OAuth dev metadata for controlled testing.",
      target: "client",
      command:
        "codencer activation official --gateway https://mcp.codencer.dev --project codencer --token-env CODENCER_GATEWAY_MCP_TOKEN --json",
    },
    {
      id: "curl",
      title: "Gateway curl smoke",
      description: "Runs MCP initialize/tools/list against Gateway.",
      target: "gateway",
      command:
        "curl -fsS https://mcp.codencer.dev/mcp -H 'Authorization: Bearer $CODENCER_GATEWAY_MCP_TOKEN'",
    },
  ],
};
