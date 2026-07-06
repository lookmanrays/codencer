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
import { RunListResponseSchema } from "@/schemas/run-history";
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
    expect(demoSnapshot.runs[0]?.runId).toBe("run_demo_console");
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
    expect(AuditEventListResponseSchema.parse({ events: null }).groups).toEqual(
      [],
    );
    expect(
      AuditEventListResponseSchema.parse({ events: null }).pagination.has_more,
    ).toBe(false);
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
    expect(RunListResponseSchema.parse({ runs: null }).runs).toEqual([]);
    expect(
      RunListResponseSchema.parse({ runs: null }).pagination.has_more,
    ).toBe(false);
  });

  it("parses audit pagination and grouped lifecycle summaries", () => {
    const parsed = AuditEventListResponseSchema.parse({
      audit_events: [
        {
          actor_user_id: "user_1",
          created_at: "2026-06-24T12:00:00Z",
          id: "evt_1",
          metadata: {
            project_id: "codencer",
            run_history_id: "hist_1",
            run_id: "run_1",
          },
          summary: "Run completed",
          type: "run_completed",
        },
      ],
      groups: [
        {
          event_count: 8,
          first_event_at: "2026-06-24T11:59:00Z",
          id: "run:hist_1",
          last_event_at: "2026-06-24T12:00:00Z",
          project_id: "codencer",
          run_history_id: "hist_1",
          run_id: "run_1",
          summary: "8 lifecycle events for run run_1",
          types: ["task_submitted", "run_completed"],
        },
      ],
      pagination: {
        has_more: true,
        limit: 1,
        next_offset: 1,
        offset: 0,
      },
    });
    expect(parsed.auditEvents[0].runHistoryId).toBe("hist_1");
    expect(parsed.groups[0]).toMatchObject({
      eventCount: 8,
      projectId: "codencer",
      runHistoryId: "hist_1",
      runId: "run_1",
    });
    expect(parsed.pagination).toMatchObject({
      has_more: true,
      limit: 1,
      next_offset: 1,
      offset: 0,
    });
  });

  it("extracts fallback result text from Gateway run reports", () => {
    const parsed = RunSubmitResponseSchema.parse({
      ok: true,
      project_id: "codencer",
      run_history_id: "runhist_1",
      result: {
        ok: true,
        run_id: "run-1",
        status: "completed",
        tasks: [
          {
            evidence: {
              result: {
                raw_output: "Executor returned README summary.",
              },
            },
          },
        ],
      },
    });
    expect(parsed.runHistoryId).toBe("runhist_1");
    expect(parsed.summary).toContain("Executor returned README summary");
  });

  it("extracts execution mode from run report simulation metadata", () => {
    expect(
      RunSubmitResponseSchema.parse({
        ok: true,
        project_id: "codencer",
        result: {
          ok: true,
          run_id: "run-real",
          tasks: [
            {
              adapter: "codex",
              profile: "codex-workspace",
              evidence: {
                result: {
                  is_simulation: false,
                  raw_output: "Real Codex output.",
                },
              },
            },
          ],
        },
      }).executionMode,
    ).toBe("real");
    expect(
      RunSubmitResponseSchema.parse({
        ok: true,
        project_id: "codencer",
        result: {
          ok: true,
          run_id: "run-sim",
          evidence: {
            result: {
              is_simulation: true,
              raw_output: "Simulated successful codex task.",
            },
          },
        },
      }).executionMode,
    ).toBe("simulation");
    expect(
      RunSubmitResponseSchema.parse({
        ok: true,
        project_id: "codencer",
        result: { ok: true, run_id: "run-unknown" },
      }).executionMode,
    ).toBe("unknown");
    expect(
      RunListResponseSchema.parse({
        runs: [
          {
            created_at: "2026-06-20T12:00:00Z",
            id: "runhist-real",
            project_id: "codencer",
            report: {
              evidence: {
                result: {
                  is_simulation: false,
                  raw_output: "Real Codex output.",
                },
              },
            },
            scope: "gateway_submitted",
            updated_at: "2026-06-20T12:00:01Z",
          },
        ],
      }).runs[0]?.executionMode,
    ).toBe("real");
    expect(
      RunListResponseSchema.parse({
        runs: [
          {
            created_at: "2026-06-20T12:00:00Z",
            id: "runhist-scope",
            project_id: "codencer",
            scope: "gateway_submitted",
            updated_at: "2026-06-20T12:00:01Z",
          },
        ],
      }).runs[0]?.scope,
    ).toBe("gateway_submitted");
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
