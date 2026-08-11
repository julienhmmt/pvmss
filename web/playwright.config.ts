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
			SESSION_SECRET: 'e2e-session-secret-with-at-least-thirty-two-bytes',
			// Short guard so nodes.spec.ts can exercise "wait it out, click again —
			// it works" (quickstart.md step 6) without a multi-second sleep.
			PVMSS_V04_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL: '2s',
			// bcrypt hash of "pvmss-e2e-admin" (bcrypt.MinCost) — admin-catalog.spec.ts
			// (T11) is the only spec that needs POST /api/v1/auth/admin-login.
			ADMIN_PASSWORD_HASH: '$2a$04$W8Da6bYNWJPztehxCYwifOoPpt0GfHtyMKGBtgy2PWkcCI7In6nee',
		},
		url: 'http://127.0.0.1:50001/health',
		reuseExistingServer: !process.env.CI,
		timeout: 120 * 1000,
	},
});
