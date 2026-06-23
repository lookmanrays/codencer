import { describe, expect, it } from "vitest";
import { demoSnapshot } from "@/api/demo-data";
import {
  ActivationCommandListResponseSchema,
  ActivationCommandSchema,
} from "@/schemas/activation";
import {
  AuditEventListResponseSchema,
  AuditEventSchema,
} from "@/schemas/audit";
import {
  ConnectorListResponseSchema,
  ConnectorSchema,
} from "@/schemas/connectors";
import {
  ExecutorListResponseSchema,
  ExecutorProfileSchema,
} from "@/schemas/executors";
import { MachineListResponseSchema, MachineSchema } from "@/schemas/machines";
import { ProjectListResponseSchema, ProjectSchema } from "@/schemas/projects";
import { RelayListResponseSchema, RelayProfileSchema } from "@/schemas/relays";
import { RunSubmitResponseSchema } from "@/schemas/runs";
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
    expect(ExecutorProfileSchema.parse(demoSnapshot.executors[0]).id).toBe(
      "claude-default",
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

  it("rejects unsafe Gateway run output shapes", () => {
    expect(() =>
      RunSubmitResponseSchema.parse({
        ok: true,
        project_id: "codencer",
        result: {
          ok: true,
          report_path: "/Users/operator/.codencer-live-test/report.json",
        },
      }),
    ).toThrow();
  });

  it("normalizes null or missing Gateway collection fields defensively", () => {
    expect(
      MachineListResponseSchema.parse({ machines: null }).machines,
    ).toEqual([]);
    expect(MachineListResponseSchema.parse({}).machines).toEqual([]);
    expect(
      ConnectorListResponseSchema.parse({ connectors: null }).connectors,
    ).toEqual([]);
    expect(
      ExecutorListResponseSchema.parse({ executors: null }).executors,
    ).toEqual([]);
    expect(
      ProjectListResponseSchema.parse({
        projects: null,
        relay_errors: null,
      }),
    ).toMatchObject({ projects: [], relayErrors: [] });
    expect(
      AuditEventListResponseSchema.parse({ audit_events: null }).auditEvents,
    ).toEqual([]);
    expect(
      AuditEventListResponseSchema.parse({ events: null }).auditEvents,
    ).toEqual([]);
    expect(
      ActivationCommandListResponseSchema.parse({
        activation_commands: null,
      }).activationCommands,
    ).toEqual([]);
    expect(
      ActivationCommandListResponseSchema.parse({ commands: null })
        .activationCommands,
    ).toEqual([]);
    expect(RelayListResponseSchema.parse({ relays: null }).relays).toEqual([]);
  });

  it("maps relay-derived machine statuses to dashboard states", () => {
    expect(
      MachineListResponseSchema.parse({
        machines: [{ id: "mach-1", status: "online" }],
      }).machines[0]?.status,
    ).toBe("online");
    expect(
      MachineListResponseSchema.parse({
        machines: [{ id: "mach-2", status: "offline" }],
      }).machines[0]?.status,
    ).toBe("offline");
  });
});
