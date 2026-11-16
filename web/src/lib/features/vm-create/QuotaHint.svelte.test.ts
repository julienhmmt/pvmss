import { afterEach, describe, expect, it } from 'vitest';
import { flushSync, mount } from 'svelte';
import QuotaHint from './QuotaHint.svelte';
import { m } from '#lib/paraglide/messages.js';

function hint(): HTMLElement | null {
	return document.querySelector('[data-testid="quota-hint"]');
}

describe('QuotaHint', () => {
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('says which machine this will be and what is left', () => {
		mount(QuotaHint, { target: document.body, props: { quota: { used: 3, allowed: 6 } } });
		flushSync();
		expect(hint()?.textContent).toContain(m['vms.create.quotaHint']({ next: 4, allowed: 6, left: 2 }));
		expect(document.querySelector('[role="meter"]')?.getAttribute('aria-valuenow')).toBe('4');
	});

	it('stays hidden for unlimited, full, or unknown quotas', () => {
		for (const quota of [{ used: 2, allowed: -1 }, { used: 6, allowed: 6 }, undefined]) {
			mount(QuotaHint, { target: document.body, props: { quota } });
		}
		flushSync();
		expect(hint()).toBeNull();
	});

	it('drops the segmented meter for large allowances', () => {
		mount(QuotaHint, { target: document.body, props: { quota: { used: 1, allowed: 100 } } });
		flushSync();
		expect(hint()).not.toBeNull();
		expect(document.querySelector('[role="meter"]')).toBeNull();
	});
});
