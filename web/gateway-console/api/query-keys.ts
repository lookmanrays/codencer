export const queryKeys = {
  activationCommands: ["activation-commands"] as const,
  auditEvents: ["audit-events"] as const,
  connectors: ["connectors"] as const,
  machines: ["machines"] as const,
  project: (projectId: string) => ["projects", projectId] as const,
  projectRunReport: (projectId: string, runId: string) =>
    ["projects", projectId, "runs", runId, "report"] as const,
  projects: ["projects"] as const,
  relay: (relayId: string) => ["relays", relayId] as const,
  relayHealth: (relayId: string) => ["relays", relayId, "health"] as const,
  relays: ["relays"] as const,
  workspace: ["workspace"] as const,
};
