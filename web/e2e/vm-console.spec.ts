import { test, expect, type APIRequestContext } from '@playwright/test';

async function signInAlice(request: APIRequestContext): Promise<void> {
	const response = await request.post('/api/v1/auth/login', {
		data: { username: 'alice', password: 'pvmss-alice' }
	});
	expect(response.status()).toBe(200);
}

test.describe('T10 VM console VNC', () => {
	test.describe.configure({ mode: 'serial' });

	test('P1: open a console and see the fake screen', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms/default/100');

		// The console banner is visible on the VM detail page.
		await expect(page.getByTestId('vm-console-open')).toBeVisible();
		await page.getByTestId('vm-console-open').click();

		// The console route loads.
		await expect(page).toHaveURL(/\/vms\/default\/100\/console$/);
		await expect(page.getByTestId('vm-console-title')).toContainText('VM 100 Console');

		// The status badge transitions from connecting to connected — the fake
		// RFB server completes the handshake immediately, so this should happen
		// within a few seconds.
		await expect(page.getByTestId('vm-console-status')).toContainText(/connected|connecting/, { timeout: 10000 });
		// Wait for the connected state specifically.
		await expect(page.getByTestId('vm-console-status')).toContainText('connected', { timeout: 15000 });
	});

	test('P2: toolbar controls are visible and the scale toggle works', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms/default/100/console');

		// Wait for connection.
		await expect(page.getByTestId('vm-console-status')).toContainText('connected', { timeout: 15000 });

		// The toolbar is visible with all controls.
		await expect(page.getByTestId('vm-console-toolbar')).toBeVisible();
		await expect(page.getByTestId('vm-console-scale')).toContainText('Scale: On');
		await expect(page.getByTestId('vm-console-ctrlaltdel')).toBeVisible();
		await expect(page.getByTestId('vm-console-disconnect')).toBeVisible();

		// Toggle scale off.
		await page.getByTestId('vm-console-scale').click();
		await expect(page.getByTestId('vm-console-scale')).toContainText('Scale: Off');

		// Toggle back on.
		await page.getByTestId('vm-console-scale').click();
		await expect(page.getByTestId('vm-console-scale')).toContainText('Scale: On');
	});

	test('P2: disconnect and reconnect', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms/default/100/console');

		await expect(page.getByTestId('vm-console-status')).toContainText('connected', { timeout: 15000 });

		// Disconnect.
		await page.getByTestId('vm-console-disconnect').click();
		await expect(page.getByTestId('vm-console-status')).toContainText('disconnected');
		await expect(page.getByTestId('vm-console-disconnected')).toBeVisible();

		// Reconnect.
		await page.getByTestId('vm-console-reconnect').click();
		await expect(page.getByTestId('vm-console-status')).toContainText('connected', { timeout: 15000 });
	});

	test('P3: clipboard from VM is visible (SC-005)', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms/default/100/console');

		await expect(page.getByTestId('vm-console-status')).toContainText('connected', { timeout: 15000 });

		// The fake RFB server sends a ServerCutText right after ServerInit with
		// the fixture string "hello from the fake console". noVNC dispatches a
		// clipboard event, which the store surfaces as the clipboard preview.
		await expect(page.getByTestId('vm-console-clipboard-preview')).toContainText('hello from the fake console', { timeout: 10000 });

		// The "Copy from VM" button is enabled because the clipboard has content.
		await expect(page.getByTestId('vm-console-copy-from-vm')).toBeEnabled();
	});

	test('SC-002: a non-owner cannot obtain a console ticket', async ({ request }) => {
		// Bob cannot open a console for VM 100 (owned by alice).
		await request.post('/api/v1/auth/login', { data: { username: 'bob', password: 'pvmss-bob' } });
		const response = await request.post('/api/v1/vms/default/100/vnc-ticket');
		expect(response.status()).toBe(403);
		const body = await response.json();
		expect(body.code).toBe('forbidden');
	});

	test('T038: full open → scale → Ctrl+Alt+Del → clipboard → disconnect → reconnect sequence', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms/default/100/console');

		// 1. Open — the fake screen connects.
		await expect(page.getByTestId('vm-console-status')).toContainText('connected', { timeout: 15000 });

		// 2. Scale toggle.
		await expect(page.getByTestId('vm-console-scale')).toContainText('Scale: On');
		await page.getByTestId('vm-console-scale').click();
		await expect(page.getByTestId('vm-console-scale')).toContainText('Scale: Off');
		await page.getByTestId('vm-console-scale').click();
		await expect(page.getByTestId('vm-console-scale')).toContainText('Scale: On');

		// 3. Ctrl+Alt+Del — the fake server accepts it without closing.
		await page.getByTestId('vm-console-ctrlaltdel').click();
		await expect(page.getByTestId('vm-console-status')).toContainText('connected');

		// 4. Clipboard — the fake server's ServerCutText surfaces.
		await expect(page.getByTestId('vm-console-clipboard-preview')).toContainText('hello from the fake console', { timeout: 10000 });

		// 5. Disconnect.
		await page.getByTestId('vm-console-disconnect').click();
		await expect(page.getByTestId('vm-console-status')).toContainText('disconnected');

		// 6. Reconnect — a fresh ticket is requested.
		await page.getByTestId('vm-console-reconnect').click();
		await expect(page.getByTestId('vm-console-status')).toContainText('connected', { timeout: 15000 });
	});

	test('T030: clean disconnect keeps the route mounted — the boundary fallback is NOT shown', async ({ page }) => {
		await signInAlice(page.request);
		await page.goto('/vms/default/100/console');

		await expect(page.getByTestId('vm-console-status')).toContainText('connected', { timeout: 15000 });

		// A clean, expected disconnect via the toolbar button.
		await page.getByTestId('vm-console-disconnect').click();
		await expect(page.getByTestId('vm-console-status')).toContainText('disconnected');

		// The route heading is still in the DOM — the route did not unmount.
		await expect(page.getByTestId('vm-console-title')).toContainText('VM 100 Console');

		// The disconnected fallback UI is visible, not an error page.
		await expect(page.getByTestId('vm-console-disconnected')).toBeVisible();

		// The boundary fallback is NOT shown — this was a clean disconnect,
		// not a render-time crash.
		await expect(page.getByTestId('vm-console-boundary-fallback')).toBeHidden();
	});

	test('SC-004: boundary catches a render-time error — the fallback is shown and the route stays mounted', async ({ page }) => {
		await signInAlice(page.request);

		// Set the test-only global before navigation so VmConsole.svelte throws
		// during component initialization — a render-time error that the
		// <svelte:boundary> in +page.svelte must catch (SC-004, AC04 §4).
		// Svelte 5 boundaries only catch render/effect errors, not event-
		// handler or async errors, so this is the only reliable way to
		// exercise the failed snippet. Deliberately not a URL query parameter —
		// see VmConsole.svelte's comment on why.
		await page.addInitScript(() => {
			window.__pvmssForceConsoleBoundaryError = true;
		});
		await page.goto('/vms/default/100/console');

		// The boundary fallback IS shown — the render-time throw was caught.
		await expect(page.getByTestId('vm-console-boundary-fallback')).toBeVisible({ timeout: 10000 });

		// The route heading is still in the DOM — the surrounding route did
		// not unmount. This is the core SC-004 guarantee: the boundary walls
		// off the failure, the rest of the page stays intact.
		await expect(page.getByTestId('vm-console-title')).toContainText('VM 100 Console');
	});
});
