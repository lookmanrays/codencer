import { z } from "zod";

export const UserSchema = z.object({
  id: z.string(),
  email: z.string().email(),
  displayName: z.string(),
});

export const WorkspaceSchema = z.object({
  id: z.string(),
  name: z.string(),
  slug: z.string(),
  mode: z.enum(["mock", "self_host", "official_preprod"]),
});

export const RelayProfileSchema = z.object({
  id: z.string(),
  name: z.string(),
  type: z.enum(["managed", "self_host"]),
  url: z.string().url(),
  tokenRef: z.string(),
  enabled: z.boolean(),
  status: z.enum(["available", "unavailable", "disabled", "checking"]),
});

export const RelayHealthSchema = z.object({
  relayProfileId: z.string(),
  status: z.enum(["available", "unavailable", "disabled", "checking"]),
  latencyMs: z.number().nullable(),
  checkedAt: z.string(),
});

export const MachineSchema = z.object({
  id: z.string(),
  hostname: z.string(),
  hostLabel: z.string(),
  os: z.string(),
  arch: z.string(),
  status: z.enum(["online", "offline", "unknown"]),
});

export const ConnectorSchema = z.object({
  id: z.string(),
  machineId: z.string(),
  relayProfileId: z.string(),
  label: z.string(),
  status: z.enum(["online", "offline", "pending"]),
  lastSeen: z.string(),
});

export const ProjectLocationSchema = z.object({
  id: z.string(),
  projectId: z.string(),
  relayProfileId: z.string(),
  machineId: z.string(),
  hostLabel: z.string(),
  repoLabel: z.string(),
  repoHash: z.string(),
  status: z.enum(["online", "offline"]),
  ambiguity: z.enum(["none", "relay_profile", "machine_location"]),
});

export const ProjectSchema = z.object({
  id: z.string(),
  name: z.string(),
  adapter: z.string(),
  profile: z.string(),
  locations: z.array(ProjectLocationSchema),
});

export const AuditEventSchema = z.object({
  id: z.string(),
  type: z.string(),
  summary: z.string(),
  actor: z.string(),
  createdAt: z.string(),
  severity: z.enum(["info", "warning", "error"]),
});

export const DeviceCodeSchema = z.object({
  userCode: z.string().min(4, "Enter a device code like ABCD-EFGH"),
  email: z.string().email().optional(),
  displayName: z.string().optional(),
});

export const OAuthConsentSchema = z.object({
  clientId: z.string(),
  clientName: z.string(),
  workspaceId: z.string(),
  resource: z.string().url(),
  scopes: z.array(z.string()),
  operatorCode: z.string().min(6, "Approval code is required"),
});

export const ActivationCommandSchema = z.object({
  id: z.string(),
  title: z.string(),
  description: z.string(),
  command: z.string(),
  target: z.enum(["gateway", "local", "client"]),
});

export const GatewayStatusResponseSchema = z.object({
  ok: z.boolean(),
  service: z.string(),
  mcp_url: z.string().optional(),
  public_base_url: z.string().optional(),
});

export const RelayListResponseSchema = z.object({
  relays: z.array(
    z.object({
      id: z.string(),
      name: z.string(),
      type: z.string(),
      url: z.string(),
      enabled: z.boolean(),
      status: z.string().optional(),
      token_env: z.string().optional(),
      token_file: z.string().optional(),
    }),
  ),
});

export const ConsoleSnapshotSchema = z.object({
  user: UserSchema,
  workspace: WorkspaceSchema,
  mcpEndpoint: z.string().url(),
  relays: z.array(RelayProfileSchema),
  relayHealth: z.array(RelayHealthSchema),
  machines: z.array(MachineSchema),
  connectors: z.array(ConnectorSchema),
  projects: z.array(ProjectSchema),
  auditEvents: z.array(AuditEventSchema),
  activationCommands: z.array(ActivationCommandSchema),
});

export type User = z.infer<typeof UserSchema>;
export type Workspace = z.infer<typeof WorkspaceSchema>;
export type RelayProfile = z.infer<typeof RelayProfileSchema>;
export type RelayHealth = z.infer<typeof RelayHealthSchema>;
export type Machine = z.infer<typeof MachineSchema>;
export type Connector = z.infer<typeof ConnectorSchema>;
export type Project = z.infer<typeof ProjectSchema>;
export type ProjectLocation = z.infer<typeof ProjectLocationSchema>;
export type AuditEvent = z.infer<typeof AuditEventSchema>;
export type DeviceCode = z.infer<typeof DeviceCodeSchema>;
export type OAuthConsent = z.infer<typeof OAuthConsentSchema>;
export type ActivationCommand = z.infer<typeof ActivationCommandSchema>;
export type ConsoleSnapshot = z.infer<typeof ConsoleSnapshotSchema>;
