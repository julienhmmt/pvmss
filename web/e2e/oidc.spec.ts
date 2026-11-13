import { test, expect, type APIRequestContext } from '@playwright/test';
import { csrfHeaders } from './support/csrf';

// The fake cluster simulates the Proxmox OpenID realm: its "IdP" redirects
// straight back to the callback, signing in as alice@pve. No real IdP needed.
async function setRealm(request: APIRequestContext, realm: string): Promise<void> {
	const login = await request.post('/api/v1/auth/admin-login', { data: { password: 'pvmss-e2e-admin' } });
	expect(login.status()).toBe(200);
	const clusters = (await (await request.get('/api/v1/admin/clusters')).json()) as { name: string; displayName: string; url: string; tokenId: string; tlsInsecureSkipVerify: boolean }[];
	const cluster = clusters.find((c) => c.name === 'default');
	if (!cluster) throw new Error('default cluster missing');
	const update = await request.put('/api/v1/admin/clusters/default', {
		headers: await csrfHeaders(request),
		data: { displayName: cluster.displayName, url: cluster.url, tokenId: cluster.tokenId, tlsInsecureSkipVerify: cluster.tlsInsecureSkipVerify, tokenSecret: '', oidcRealm: realm }
	});
	expect(update.status(), await update.text()).toBe(200);
	await request.post('/api/v1/auth/logout', { headers: await csrfHeaders(request) });
}

test.describe.serial('SSO through the cluster OpenID realm', () => {
	test.beforeAll(async ({ playwright, baseURL }) => {
		const request = await playwright.request.newContext(baseURL ? { baseURL } : {});
		await setRealm(request, 'e2e-oidc');
		await request.dispose();
	});

	test.afterAll(async ({ playwright, baseURL }) => {
		const request = await playwright.request.newContext(baseURL ? { baseURL } : {});
		await setRealm(request, '');
		await request.dispose();
	});

	test('signs in with SSO, honours next, and hides the password form', async ({ page }) => {
		await page.goto('/login?next=%2Fprofile');
		await page.locator('#login-cluster').selectOption('default');
		await page.getByTestId('login-sso').click();

		await expect(page).toHaveURL(/\/profile$/);
		await expect(page.getByTestId('profile-sso-note')).toBeVisible();

		const me = await page.request.get('/api/v1/auth/me');
		expect(await me.json()).toMatchObject({ username: 'alice@pve', provider: 'oidc', cluster: 'default' });
	});

	test('shows a translated error when the callback fails', async ({ page }) => {
		await page.goto('/login?oidc=error&reason=oidc_cancelled');
		await expect(page.getByRole('alert')).toContainText(/cancel/i);
	});
});
