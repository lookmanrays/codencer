import fs from "node:fs";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { consoleMode } from "@/api/config";

const repoRoot = path.resolve(process.cwd(), "../..");
const packageRoot = process.cwd();

const deprecatedEnv = "NEXT_PUBLIC_CODENCER_CONSOLE_MOCKS";

const envAndDocsTargets = [
  path.join(packageRoot, ".env.example"),
  path.join(packageRoot, "README.md"),
  path.join(repoRoot, "docs/gateway-console.md"),
  path.join(repoRoot, "docs/ui"),
  path.join(repoRoot, "docs/architecture"),
];

const productSourceTargets = [
  path.join(packageRoot, "app/console"),
  path.join(packageRoot, "app/device"),
  path.join(packageRoot, "app/oauth"),
  path.join(packageRoot, "components/console"),
  path.join(packageRoot, "features/console"),
];

describe("Gateway Console architecture guards", () => {
  const originalMode = process.env.NEXT_PUBLIC_CODENCER_CONSOLE_MODE;

  afterEach(() => {
    if (originalMode === undefined) {
      delete process.env.NEXT_PUBLIC_CODENCER_CONSOLE_MODE;
    } else {
      process.env.NEXT_PUBLIC_CODENCER_CONSOLE_MODE = originalMode;
    }
  });

  it("does not document the deprecated mock env flag in product docs/config", () => {
    const offenders = scanTextFiles(envAndDocsTargets).filter((file) =>
      fs.readFileSync(file, "utf8").includes(deprecatedEnv),
    );
    expect(offenders.map(relativeToRepo)).toEqual([]);
  });

  it("keeps product routes and console modules isolated from demo fixtures", () => {
    const offenders = scanTextFiles(productSourceTargets).filter((file) =>
      fs.readFileSync(file, "utf8").includes("@/api/demo-data"),
    );
    expect(offenders.map(relativeToRepo)).toEqual([]);
  });

  it("defaults to live mode unless demo mode is explicit", () => {
    delete process.env.NEXT_PUBLIC_CODENCER_CONSOLE_MODE;
    expect(consoleMode()).toBe("live");

    process.env.NEXT_PUBLIC_CODENCER_CONSOLE_MODE = "demo";
    expect(consoleMode()).toBe("demo");
  });
});

function scanTextFiles(targets: string[]) {
  return targets.flatMap((target) => {
    const stat = fs.statSync(target);
    if (stat.isDirectory()) {
      return walk(target).filter(isScannedTextFile);
    }
    return isScannedTextFile(target) ? [target] : [];
  });
}

function walk(dir: string): string[] {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const fullPath = path.join(dir, entry.name);
    return entry.isDirectory() ? walk(fullPath) : [fullPath];
  });
}

function isScannedTextFile(file: string) {
  return [".env.example", ".md", ".ts", ".tsx"].some((suffix) =>
    file.endsWith(suffix),
  );
}

function relativeToRepo(file: string) {
  return path.relative(repoRoot, file);
}
