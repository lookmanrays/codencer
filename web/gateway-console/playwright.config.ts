import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/e2e",
  fullyParallel: true,
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: "http://127.0.0.1:19575",
    trace: "on-first-retry",
  },
  webServer: {
    command:
      "NEXT_PUBLIC_CODENCER_CONSOLE_MODE=demo npm run dev -- --hostname 127.0.0.1 --port 19575",
    url: "http://127.0.0.1:19575",
    reuseExistingServer: !process.env.CI,
    timeout: 120000,
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
