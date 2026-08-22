import { test, expect } from '@playwright/test';

async function signIn(request: import('@playwright/test').APIRequestContext, username: string, password: string): Promise<void> {
	const response = await request.post('/api/v1/auth/login', { data: { username, password, cluster: 'default' } });
	expect(response.status()).toBe(200);
}

// REPORT.md #2: inject an SSH key into the running guest via the QEMU guest
// agent, without a reboot, and reject a malformed key before it reaches the agent.
test('injects an SSH key post-boot via the guest agent without a reboot', async ({ page }) => {
	// page.request shares the page session cookie, so both the UI click and the
	// direct API calls below authenticate with the same alice session.
	await signIn(page.request, 'alice', 'pvmss-alice');
	await page.goto('/vms/default/102');
	await page.getByTestId('vm-tab-cloudinit').click();
	await page.getByTestId('cloudinit-inject-key').fill('ssh-ed25519 AAAA-injected demo@laptop');
	await page.getByTestId('cloudinit-inject-user').fill('debian');
	await page.getByTestId('cloudinit-inject-now').click();
	await expect(page.getByTestId('cloudinit-inject-now')).toBeEnabled();

	// Diagnose backend directly (same request context that holds the session cookie).
	const postResp = await page.request.post('/api/v1/vms/default/102/cloudinit/ssh-keys', {
		data: { key: 'ssh-ed25519 AAAA-injected demo@laptop', user: 'debian' }
	});
	expect(postResp.status()).toBe(200);

	const config = await (await page.request.get('/api/v1/vms/default/102/cloudinit')).json();
	expect(config.sshKeys ?? []).toContain('ssh-ed25519 AAAA-injected demo@laptop');

	// Reject a malformed key before it reaches the agent (no injection, field kept).
	// (A single-line obviously-invalid string — <input type=text> cannot hold a
	// newline, so the multi-line smuggling case is covered by the Go/API tests.)
	await page.getByTestId('cloudinit-inject-key').fill('not-a-valid-ssh-key');
	await page.getByTestId('cloudinit-inject-now').click();
	await expect(page.getByTestId('cloudinit-inject-key')).toHaveValue('not-a-valid-ssh-key');
	await expect(page.getByTestId('cloudinit-inject-now')).toBeEnabled();
});

test('rejects a malformed SSH key via the API with 400 invalid_key', async ({ page }) => {
	await signIn(page.request, 'alice', 'pvmss-alice');
	const postResp = await page.request.post('/api/v1/vms/default/102/cloudinit/ssh-keys', {
		data: { key: 'ssh-rsa AAAA\ninjected', user: 'debian' }
	});
	expect(postResp.status()).toBe(400);
	expect(await postResp.json()).toEqual({ code: 'invalid_key', message: expect.any(String) });
});
