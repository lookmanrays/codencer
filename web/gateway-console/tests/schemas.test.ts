import { describe, expect, it } from "vitest";
import { demoSnapshot } from "@/api/demo-data";
import { ActivationCommandSchema } from "@/schemas/activation";
import { AuditEventSchema } from "@/schemas/audit";
import { ConnectorSchema } from "@/schemas/connectors";
import { MachineSchema } from "@/schemas/machines";
import { ProjectListResponseSchema, ProjectSchema } from "@/schemas/projects";
import { RelayListResponseSchema, RelayProfileSchema } from "@/schemas/relays";
import { WorkspaceResponseSchema, WorkspaceSchema } from "@/schemas/workspace";

describe("domain schemas", () => {
  it("validates explicit demo fixture data", () => {
    expect(WorkspaceSchema.parse(demoSnapshot.workspace).mode).toBe("demo");
    expect(RelayProfileSchema.parse(demoSnapshot.relays[0]).tokenRef).toBe(
      "CODENCER_DEFAULT_RELAY_TOKEN",
    );
    expect(MachineSchema.parse(demoSnapshot.machines[0]).status).toBe("online");
    expect(ConnectorSchema.parse(demoSnapshot.connectors[0]).status).toBe(
      "online",
    );
    expect(ProjectSchema.parse(demoSnapshot.projects[0]).id).toBe("codencer");
    expect(AuditEventSchema.parse(demoSnapshot.auditEvents[0]).severity).toBe(
      "info",
    );
    expect(
      ActivationCommandSchema.parse(demoSnapshot.activationCommands[0]).target,
    ).toBe("gateway");
  });

  it("rejects bad Gateway workspace response shapes", () => {
    expect(() =>
      WorkspaceResponseSchema.parse({
        mcp_url: "http://127.0.0.1:19090/mcp",
        mode: "live",
        public_base_url: "http://127.0.0.1:19090",
        user: { email: "not-an-email", id: "user_bad" },
        workspace: { id: "ws_bad", kind: "personal", name: "Bad" },
      }),
    ).toThrow();
  });

  it("rejects bad Gateway relay response shapes", () => {
    expect(() =>
      RelayListResponseSchema.parse({
        relays: [
          {
            enabled: true,
            id: "relay_bad",
            name: "Bad Relay",
            type: "self_host",
            url: "not-a-url",
          },
        ],
      }),
    ).toThrow();
  });

  it("rejects bad Gateway project response shapes", () => {
    expect(() =>
      ProjectListResponseSchema.parse({
        projects: [
          {
            name: "Bad Project",
            relay_profiles: [],
          },
        ],
      }),
    ).toThrow();
  });
});
