/// <reference types="node" />
// Read-only smoke test against the real Proxmox endpoint configured in ../.env.
// Run with `make web-e2e-pve`: the server uses a throwaway SQLite database,
// never the dev database.
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
	testDir: './e2e-pve',
	fullyParallel: false,
	workers: 1,
	reporter: 'list',
	use: {
		baseURL: 'http://127.0.0.1:50102',
		trace: 'retain-on-failure',
		storageState: {
			cookies: [],
			origins: [
				{
					origin: 'http://127.0.0.1:50102',
					localStorage: [{ name: 'pvmss-locale', value: 'en' }]
				}
			]
		}
	},
	projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
	webServer: {
		command: [
			'mkdir -p ../tmp',
			'rm -f ../tmp/e2e-pve.db ../tmp/e2e-pve.db-wal ../tmp/e2e-pve.db-shm',
			'set -a',
			'. ../.env',
			'test -n "$PROXMOX_URL"',
			'test -n "$PROXMOX_API_TOKEN_NAME"',
			'test -n "$PROXMOX_API_TOKEN_VALUE"',
			'unset PVMSS_TLS_CERT_FILE PVMSS_TLS_KEY_FILE PVMSS_TLS_CLIENT_CA_FILE',
			'PVMSS_PORT=50102',
			'PVMSS_CLUSTER_SOURCE=proxmox',
			'PVMSS_DB_PATH=../tmp/e2e-pve.db',
			'PVMSS_WEB_DIR=../web/build',
			'PVMSS_COOKIE_SECURE=false',
			'LOG_LEVEL=info',
			'LOG_FORMAT=json',
			'LOG_OUTPUT=stdout',
			'cd ../server && go run ./cmd/pvmss'
		].join(' && '),
		cwd: '.',
		url: 'http://127.0.0.1:50102/health',
		reuseExistingServer: false,
		timeout: 120 * 1000
	}
});
