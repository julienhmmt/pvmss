import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
	testDir: './e2e',
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 0,
	workers: process.env.CI ? 1 : undefined,
	reporter: 'list',
	use: {
		baseURL: 'http://127.0.0.1:50001',
		trace: 'on-first-retry',
	},
	projects: [
		{ name: 'chromium', use: { ...devices['Desktop Chrome'] } },
	],
	webServer: {
		command: 'cd ../server && go run ./cmd/pvmss',
		cwd: '.',
		env: {
			PVMSS_PORT: '50001',
			PVMSS_DB_PATH: '../tmp/e2e-pvmss.db',
			PVMSS_WEB_DIR: '../web/build',
			LOG_LEVEL: 'info',
			LOG_FORMAT: 'json',
			LOG_OUTPUT: 'stdout',
		},
		url: 'http://127.0.0.1:50001/health',
		reuseExistingServer: !process.env.CI,
		timeout: 120 * 1000,
	},
});
