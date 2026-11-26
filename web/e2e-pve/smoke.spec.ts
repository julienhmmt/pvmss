import { test, expect } from '@playwright/test';

const clusterRefreshTimeoutMs = 30_000;

type HealthCheck = {
	status?: string;
};

type HealthResponse = {
	status?: string;
	demoMode?: boolean;
	checks?: {
		clusters?: HealthCheck;
	};
};

type LoginCluster = {
	name?: string;
};

test.describe('real Proxmox smoke', () => {
	test('shell renders and the configured cluster becomes reachable', async ({ page, request }): Promise<void> => {
		await page.goto('/');
		await expect(page.locator('text=Proxmox VM Self-Service (PVMSS)')).toBeVisible();

		const clusters = await request.get('/api/v1/auth/clusters');
		expect(clusters.status()).toBe(200);
		const clusterChoices = (await clusters.json()) as LoginCluster[];
		expect(clusterChoices.length).toBeGreaterThan(0);

		const readHealthState = async (): Promise<string> => {
			const response = await request.get('/health');
			if (response.status() !== 200) {
				return `http-${response.status()}`;
			}
			const body = (await response.json()) as HealthResponse;
			return `${body.status ?? 'missing'}:${body.checks?.clusters?.status ?? 'missing'}:${body.demoMode ?? 'missing'}`;
		};

		await expect
			.poll(readHealthState, { timeout: clusterRefreshTimeoutMs })
			.toBe('healthy:healthy:false');
	});
});
