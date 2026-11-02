import { beforeAll, afterEach, describe, expect, it, vi } from 'vitest';
import { mount, tick } from 'svelte';
import PoolQuotaOverride from './PoolQuotaOverride.svelte';
import type { QuotaOverride } from '#lib/features/quota-overrides/quota-overrides.js';
import { setLocale } from '#lib/paraglide/runtime.js';

const override: QuotaOverride = {
	cluster: 'pve',
	pool: 'lab',
	maxVms: 12,
	updatedBy: 'admin',
	updatedAt: '2026-11-01T00:00:00Z'
};

function render(props: { override: QuotaOverride | null; error?: string | null; onSave?: (n: number) => Promise<void>; onRemove?: () => Promise<void> }) {
	mount(PoolQuotaOverride, {
		target: document.body,
		props: {
			saving: false,
			error: null,
			onSave: vi.fn(() => Promise.resolve()),
			onRemove: vi.fn(() => Promise.resolve()),
			...props
		}
	});
}

const input = (): HTMLInputElement => {
	const element = document.querySelector<HTMLInputElement>('[data-testid="pool-quota-form"] input[type="number"]');
	if (!element) throw new Error('quota input missing');
	return element;
};

describe('PoolQuotaOverride', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
		vi.clearAllMocks();
	});

	it('shows the global quota and no remove button when the pool has no override', async () => {
		render({ override: null });
		await tick();

		expect(document.querySelector('[data-testid="pool-quota-source"]')?.textContent?.trim()).toBe('Global quota');
		expect(document.querySelector('[data-testid="pool-quota-remove"]')).toBeNull();
		expect(input().value).toBe('');
	});

	it('shows the override value, prefills it and offers removal', async () => {
		render({ override });
		await tick();

		expect(document.querySelector('[data-testid="pool-quota-source"]')?.textContent?.trim()).toBe('Override: 12 VMs');
		expect(document.querySelector('[data-testid="pool-quota-remove"]')).not.toBeNull();
		expect(input().value).toBe('12');
	});

	it('saves a whole number and passes it to onSave', async () => {
		const onSave = vi.fn(() => Promise.resolve());
		render({ override: null, onSave });
		await tick();

		input().value = '7';
		input().dispatchEvent(new Event('input', { bubbles: true }));
		await tick();
		document.querySelector<HTMLFormElement>('[data-testid="pool-quota-form"]')?.dispatchEvent(new Event('submit', { cancelable: true, bubbles: true }));
		await tick();

		expect(onSave).toHaveBeenCalledWith(7);
	});

	it('rejects a blank, fractional or negative cap without calling onSave', async () => {
		const onSave = vi.fn(() => Promise.resolve());
		render({ override: null, onSave });
		await tick();
		const form = document.querySelector<HTMLFormElement>('[data-testid="pool-quota-form"]');

		for (const value of ['', '1.5', '-3']) {
			input().value = value;
			input().dispatchEvent(new Event('input', { bubbles: true }));
			await tick();
			form?.dispatchEvent(new Event('submit', { cancelable: true, bubbles: true }));
			await tick();
		}

		expect(onSave).not.toHaveBeenCalled();
		expect(document.body.textContent).toContain('whole number');
	});

	it('removes the override on request', async () => {
		const onRemove = vi.fn(() => Promise.resolve());
		render({ override, onRemove });
		await tick();

		document.querySelector<HTMLButtonElement>('[data-testid="pool-quota-remove"]')?.click();
		await tick();

		expect(onRemove).toHaveBeenCalledTimes(1);
	});

	it('shows a store error next to the form', async () => {
		render({ override, error: 'The override could not be saved.' });
		await tick();

		expect(document.body.textContent).toContain('The override could not be saved.');
	});
});
