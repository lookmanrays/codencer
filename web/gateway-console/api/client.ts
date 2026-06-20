import { z } from "zod";
import {
  ConsoleSnapshotSchema,
  RelayListResponseSchema,
  type ConsoleSnapshot,
} from "@/schemas/console";
import { mockSnapshot } from "@/api/mock-data";

const useMocks = process.env.NEXT_PUBLIC_CODENCER_CONSOLE_MOCKS !== "false";
const gatewayBase =
  process.env.NEXT_PUBLIC_CODENCER_GATEWAY_API_BASE ?? "http://127.0.0.1:19090";

export function isMockMode() {
  return useMocks;
}

async function fetchJSON<T>(
  path: string,
  schema: z.ZodSchema<T>,
  init?: RequestInit,
): Promise<T> {
  const response = await fetch(`${gatewayBase}${path}`, {
    ...init,
    headers: {
      Accept: "application/json",
      ...(init?.headers ?? {}),
    },
  });
  if (!response.ok) {
    throw new Error(`Gateway API ${path} returned ${response.status}`);
  }
  return schema.parse(await response.json());
}

export async function getConsoleSnapshot(): Promise<ConsoleSnapshot> {
  if (useMocks) {
    await new Promise((resolve) => window.setTimeout(resolve, 20));
    return ConsoleSnapshotSchema.parse(mockSnapshot);
  }

  const relays = await fetchJSON(
    "/api/gateway/v1/relays",
    RelayListResponseSchema,
  );
  return ConsoleSnapshotSchema.parse({
    ...mockSnapshot,
    workspace: { ...mockSnapshot.workspace, mode: "self_host" },
    relays: relays.relays.map((relay) => ({
      id: relay.id,
      name: relay.name,
      type: relay.type === "managed" ? "managed" : "self_host",
      url: relay.url,
      tokenRef: relay.token_env ?? relay.token_file ?? "server-side",
      enabled: relay.enabled,
      status: relay.enabled
        ? ((relay.status as "available" | "unavailable") ?? "checking")
        : "disabled",
    })),
  });
}
