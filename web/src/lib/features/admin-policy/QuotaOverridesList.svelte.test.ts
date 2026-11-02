import { beforeAll, afterEach, describe, expect, it } from 'vitest';
import { mount, tick } from 'svelte';
import QuotaOverridesList from './QuotaOverridesList.svelte';
import { setLocale } from '#lib/paraglide/runtime.js';

describe('QuotaOverridesList', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('lists each override with a link to its pool detail page', async () => {
		mount(QuotaOverridesList, {
			target: document.body,
			props: {
				error: null,
				overrides: [{ cluster: 'pve 1', pool: 'lab', maxVms: 12, updatedBy: 'admin', updatedAt: '2026-11-01T00:00:00Z' }]
			}
		});
		await tick();

		const link = document.querySelector<HTMLAnchorElement>('[data-testid="quota-overrides"] a');
		expect(link?.textContent?.trim()).toBe('lab');
		expect(link?.getAttribute('href')).toBe('/admin/pools/lab?cluster=pve%201');
		expect(document.body.textContent).toContain('12 VMs');
	});

	it('shows an empty state when no pool has an override', async () => {
		mount(QuotaOverridesList, { target: document.body, props: { overrides: [], error: null } });
		await tick();

		expect(document.querySelector('[data-testid="quota-overrides-empty"]')?.textContent).toContain('global quota');
	});

	it('shows the load error instead of the list', async () => {
		mount(QuotaOverridesList, { target: document.body, props: { overrides: [], error: 'Pool overrides could not be loaded.' } });
		await tick();

		expect(document.body.textContent).toContain('Pool overrides could not be loaded.');
		expect(document.querySelector('[data-testid="quota-overrides-empty"]')).toBeNull();
	});
});
