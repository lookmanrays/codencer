export type ConsoleMode = "live" | "demo";

export function consoleMode(): ConsoleMode {
  return process.env.NEXT_PUBLIC_CODENCER_CONSOLE_MODE === "demo"
    ? "demo"
    : "live";
}

export function isDemoMode() {
  return consoleMode() === "demo";
}
