import { describe, it, expect } from 'vitest';
import { m } from '$lib/paraglide/messages.js';
import { setLocale } from '$lib/paraglide/runtime.js';

describe('machine-count plurals', () => {
	it('EN: 1 machine, 2 machines', async () => {
		await setLocale('en', { reload: false });
		expect(m['vms.list.allowanceUnlimited']({ used: 1 })).toBe('1 machine, no limit set');
		expect(m['vms.list.allowanceUnlimited']({ used: 2 })).toBe('2 machines, no limit set');
		expect(m['vms.create.summary.quotaUnlimited']({ used: 1 })).toBe('After creation: 1 machine');
	});

	it('FR: 1 machine, 2 machines', async () => {
		await setLocale('fr', { reload: false });
		expect(m['vms.list.allowanceUnlimited']({ used: 1 })).toBe('1 machine, aucune limite');
		expect(m['vms.list.allowanceUsed']({ used: 1, allowed: 1 })).toBe('1 / 1 machine utilisée');
		expect(m['chrome.sidebar.quotaUnlimited']({ used: 2 })).toBe('2 machines · sans plafond');
	});
});
