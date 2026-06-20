import { describe, expect, it } from "vitest";
import { ConsoleSnapshotSchema } from "@/schemas/console";
import { mockSnapshot } from "@/api/mock-data";

describe("ConsoleSnapshotSchema", () => {
  it("validates seeded console data", () => {
    const parsed = ConsoleSnapshotSchema.parse(mockSnapshot);
    expect(parsed.mcpEndpoint).toBe("https://mcp.codencer.dev/mcp");
    expect(parsed.relays[0]?.tokenRef).toBe("CODENCER_DEFAULT_RELAY_TOKEN");
  });
});
