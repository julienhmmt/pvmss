import { describe, it, expect, vi, afterEach } from 'vitest';
import { mount, tick, unmount } from 'svelte';
import ResetPoolPasswordDialog from './ResetPoolPasswordDialog.svelte';
import type { ResetPoolCredentials } from './pool-detail.svelte';

const CREDENTIALS: ResetPoolCredentials = { username: 'pvmss-alice@pve', password: 'generated-1' };

let mounted: Record<string, never> | undefined;

function buildProps(overrides: Record<string, unknown> = {}) {
	return {
		target: document.body,
		props: {
			open: true,
			username: 'pvmss-alice@pve',
			saving: false,
			onClose: () => {},
			onReset: () => Promise.resolve(CREDENTIALS),
			...overrides
		}
	};
}

async function render(overrides: Record<string, unknown> = {}): Promise<void> {
	mounted = mount(ResetPoolPasswordDialog, buildProps(overrides));
	await tick();
}

afterEach(async () => {
	if (mounted) await unmount(mounted);
	mounted = undefined;
	document.body.innerHTML = '';
});

function passwordInputs(): NodeListOf<HTMLInputElement> {
	return document.querySelectorAll<HTMLInputElement>('input[type="password"]');
}

function passwordInput(index: number): HTMLInputElement {
	const input = passwordInputs().item(index);
	if (!input) throw new Error(`password input ${index} missing`);
	return input;
}

function setValue(input: HTMLInputElement, value: string): void {
	input.value = value;
	input.dispatchEvent(new Event('input', { bubbles: true }));
}

async function submit(): Promise<void> {
	document
		.querySelector<HTMLElement>('[data-testid="pool-reset-submit"]')
		?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
	await new Promise((resolve) => setTimeout(resolve, 0));
	await tick();
}

async function toggleCustom(): Promise<void> {
	const checkbox = document.querySelector<HTMLInputElement>('input[type="checkbox"]');
	if (!checkbox) return;
	checkbox.checked = true;
	checkbox.dispatchEvent(new Event('change', { bubbles: true }));
	await tick();
}

describe('ResetPoolPasswordDialog', () => {
	it('defaults to the generated mode with the session warning and no password fields', async () => {
		await render();
		const text = document.body.textContent ?? '';
		expect(text).toContain('pvmss-alice@pve');
		expect(passwordInputs().length).toBe(0);
		expect(document.querySelector('[data-testid="pool-reset-submit"]')).not.toBeNull();
	});

	it('sends an empty password when generated mode is submitted', async () => {
		const onReset = vi.fn(() => Promise.resolve(CREDENTIALS));
		await render({ onReset });

		await submit();

		expect(onReset).toHaveBeenCalledWith('');
	});

	it('blocks a weak or mismatched custom password before any request', async () => {
		const onReset = vi.fn(() => Promise.resolve(CREDENTIALS));
		await render({ onReset });

		await toggleCustom();
		expect(passwordInputs().length).toBe(2);

		setValue(passwordInput(0), 'short');
		setValue(passwordInput(1), 'short');
		await tick();
		await submit();
		expect(onReset).not.toHaveBeenCalled();

		setValue(passwordInput(0), 'supplied-secret-1');
		setValue(passwordInput(1), 'different-secret-1');
		await tick();
		await submit();
		expect(onReset).not.toHaveBeenCalled();
	});

	it('sends the custom password when both fields match', async () => {
		const onReset = vi.fn(() => Promise.resolve(CREDENTIALS));
		await render({ onReset });

		await toggleCustom();

		setValue(passwordInput(0), 'supplied-secret-1');
		setValue(passwordInput(1), 'supplied-secret-1');
		await tick();
		await submit();

		expect(onReset).toHaveBeenCalledWith('supplied-secret-1');
	});

	it('swaps to the one-time credentials view on success', async () => {
		await render();

		await submit();

		const text = document.body.textContent ?? '';
		expect(text).toContain('pvmss-alice@pve');
		expect(text).toContain('generated-1');
		expect(document.querySelector('[data-testid="pool-reset-submit"]')).toBeNull();
	});

	it('keeps the form open when the reset fails', async () => {
		const onReset = vi.fn(() => Promise.reject(new Error('pool password reset failed')));
		await render({ onReset });

		await submit();

		const text = document.body.textContent ?? '';
		expect(text).toContain('pool password reset failed');
		expect(document.querySelector('[data-testid="pool-reset-submit"]')).not.toBeNull();
	});
});
