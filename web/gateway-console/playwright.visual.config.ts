import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/visual",
  fullyParallel: false,
  reporter: [["list"]],
  use: {
    baseURL: "http://127.0.0.1:19575",
    trace: "retain-on-failure",
  },
  webServer: {
    command: "npm run start -- --hostname 127.0.0.1 --port 19575",
    url: "http://127.0.0.1:19575",
    reuseExistingServer: false,
    timeout: 120000,
  },
  workers: 1,
  projects: [
    {
      name: "chromium-visual",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
