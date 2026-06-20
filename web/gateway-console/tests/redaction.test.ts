import { describe, expect, it } from "vitest";
import { sanitizeForDisplay } from "@/lib/redaction";

describe("sanitizeForDisplay", () => {
  it("redacts bearer tokens and absolute paths", () => {
    const output = sanitizeForDisplay("Authorization: Bearer test-token-value /Users/example/project");
    expect(output).not.toContain("test-token-value");
    expect(output).not.toContain("/Users/example");
    expect(output).toContain("[redacted]");
    expect(output).toContain("<local-path>");
  });
});
