import { defineConfig, devices } from "@playwright/test";

// The test expects the observability stack to already be running:
//   make observability-up
//   cd frontend && bun run test:e2e:install   # first run only
//   make observability-browser-smoke-test
const baseURL = process.env.FRONTEND_URL ?? "http://localhost:8080";

export default defineConfig({
  testDir: "./e2e",
  testMatch: "**/*.pw.ts",
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  timeout: 90_000,
  expect: { timeout: 60_000 },
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "off",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
