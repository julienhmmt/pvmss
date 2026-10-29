import { test, expect } from '@playwright/test';

// The fake cluster's alice owns VM 100 and its RFB server completes the
// handshake at once, so "connected" proves the WebSocket crossed the proxy
// and stayed open long enough for RFB.
test('VNC console connects through the proxy', async ({ page }) => {
	const login = await page.request.post('/api/v1/auth/login', {
		data: { username: 'alice', password: 'pvmss-alice', cluster: 'default' }
	});
	expect(login.status()).toBe(200);

	const socket = page.waitForEvent('websocket', (ws) => ws.url().includes('/console/websocket'));
	await page.goto('/vms/default/100/console');

	const ws = await socket;
	const firstFrame = ws.waitForEvent('framereceived', { timeout: 15000 });
	await expect(page.getByTestId('vm-console-status')).toHaveAttribute('data-state', 'connected', { timeout: 15000 });
	await firstFrame;

	// Still open a moment later: catches proxies that drop the upgraded stream.
	await page.waitForTimeout(2000);
	expect(ws.isClosed()).toBe(false);
	await expect(page.getByTestId('vm-console-status')).toHaveAttribute('data-state', 'connected');
});
