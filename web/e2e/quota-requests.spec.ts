import { test, expect, type APIRequestContext, type Page } from '@playwright/test';
import { csrfHeaders } from './support/csrf';

const ADMIN_PASSWORD = 'pvmss-e2e-admin';
const ALICE_PASSWORD = 'pvmss-alice';
const MOBILE_WIDTH = 390;
const MOBILE_HEIGHT = 844;

async function signInAdmin(request: APIRequestContext, page?: Page): Promise<void> {
	const response = await request.post('/api/v1/auth/admin-login', { data: { password: ADMIN_PASSWORD } });
	expect(response.status()).toBe(200);
	if (page) {
		await page.context().addCookies((await request.storageState()).cookies);
	}
}

async function signInAlice(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/login', {
		data: { username: 'alice', password: ALICE_PASSWORD, cluster: 'default' }
	});
	expect(response.status()).toBe(200);
}

test.describe('Quota requests', () => {
	test('the decision table collapses to labelled cards on a phone', async ({ page, request }) => {
		// A finite quota is required for a request to be accepted; the global
		// limit is restored in cleanup so later specs keep the unlimited default.
		await signInAdmin(request, page);
		const policyResp = await request.put('/api/v1/admin/policy', {
			headers: await csrfHeaders(request),
			data: { cluster: 'default', quota: { maxVmPerUser: 7 } }
		});
		expect(policyResp.status()).toBe(200);

		await signInAlice(request);
		const createResp = await request.post('/api/v1/quota-requests', {
			headers: await csrfHeaders(request),
			data: { requestedQuota: 10, reason: 'e2e mobile layout check' }
		});
		expect(createResp.status()).toBe(201);

		await signInAdmin(request, page);
		try {
			await page.setViewportSize({ width: MOBILE_WIDTH, height: MOBILE_HEIGHT });
			await page.goto('/admin/quota-requests');

			const row = page.locator('[data-testid="quota-request-row"]').first();
			await expect(row).toBeVisible();

			// Collapsed layout: the row is a card, and each cell renders its
			// column label via the data-label ::before content.
			await expect(row).toHaveCSS('display', 'block');
			const requesterLabel = await row
				.locator('td[data-label]')
				.first()
				.evaluate((cell) => getComputedStyle(cell, '::before').content);
			expect(requesterLabel).toContain('Requester');

			expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(
				MOBILE_WIDTH
			);
		} finally {
			// Leave the shared fake server clean: deny pending requests and
			// restore the unlimited quota.
			const list = (await (await request.get('/api/v1/admin/quota-requests?status=pending')).json()) as {
				requests?: { id: number }[];
			};
			for (const pending of list.requests ?? []) {
				await request.post(`/api/v1/admin/quota-requests/${pending.id}/deny`, {
					headers: await csrfHeaders(request),
					data: {}
				});
			}
			await request.put('/api/v1/admin/policy', {
				headers: await csrfHeaders(request),
				data: { cluster: 'default', quota: { maxVmPerUser: -1 } }
			});
		}
	});
});
