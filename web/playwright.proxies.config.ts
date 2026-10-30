/// <reference types="node" />
// Console smoke test through every dev reverse proxy (docker-compose.dev.yml).
// Run with `make proxy-e2e`: it starts pvmss-dev on the fake cluster source
// behind the proxies, then runs this config. Chromium opens WebSockets over
// HTTP/2 extended CONNECT where the proxy offers it, which curl cannot do.
import { defineConfig, devices } from '@playwright/test';

// Dev PKI from dev/proxies/gen-cert.sh: every proxy requires this client cert.
const certs = '../dev/proxies/certs';

const proxies = [
	{ name: 'haproxy', url: 'https://localhost:50000' },
	{ name: 'nginx', url: 'https://localhost:50001' },
	{ name: 'caddy', url: 'https://localhost:50002' },
	{ name: 'traefik', url: 'https://localhost:50003' },
	// Its proxy host is created through its API by dev/proxies/npm-setup.sh.
	{ name: 'nginx-proxy-manager', url: 'https://pvmss.localhost:50004' }
];

export default defineConfig({
	testDir: './e2e-proxies',
	fullyParallel: false,
	workers: 1,
	reporter: 'list',
	use: { trace: 'retain-on-failure', ignoreHTTPSErrors: true },
	projects: proxies.map(({ name, url }) => ({
		name,
		use: {
			...devices['Desktop Chrome'],
			baseURL: url,
			clientCertificates: [{ origin: url, certPath: `${certs}/client.crt`, keyPath: `${certs}/client.key` }],
			storageState: { cookies: [], origins: [{ origin: url, localStorage: [{ name: 'pvmss-locale', value: 'en' }] }] }
		}
	}))
});
