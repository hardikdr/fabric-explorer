import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/e2e",
  workers: 1,
  retries: 0,
  timeout: 60_000,
  use: {
    baseURL: "http://127.0.0.1:8110",
    viewport: { width: 1440, height: 1000 },
    launchOptions: {
      executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
    },
    trace: "retain-on-failure",
  },
  webServer: {
    command:
      "../bin/fabric-explorer --mode fixture --listen 127.0.0.1:8110 --web-dir dist --fixture ../testdata/kubernetes/clos.json",
    url: "http://127.0.0.1:8110/readyz",
    reuseExistingServer: false,
  },
});
