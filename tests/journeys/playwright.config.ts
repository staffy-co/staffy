import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: '.',
  testMatch: '*.spec.ts',
  fullyParallel: false,
  workers: 1,
  reporter: 'line',
  use: {
    baseURL: 'http://127.0.0.1:18081',
    launchOptions: process.env.STAFFY_TEST_CHROMIUM_BIN ? { executablePath: process.env.STAFFY_TEST_CHROMIUM_BIN } : {},
    trace: 'retain-on-failure'
  },
  webServer: {
    command: 'cd ../.. && go run ./tests/journeys/fixture_server.go',
    url: 'http://127.0.0.1:18081/healthz',
    reuseExistingServer: false,
    timeout: 120_000
  }
});
