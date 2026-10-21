import { describe, expect, it } from 'vitest';
import { SessionStore } from './session.svelte';

const principal = { username: 'alice@pve', displayName: 'alice', pool: 'pool-alice', isAdmin: false, cluster: 'default', clusterDisplayName: 'Default' };

describe('SessionStore.expire', () => {
	it('drops a known principal once and reports it', () => {
		const session = new SessionStore();
		session.principal = principal;

		expect(session.expire()).toBe(true);
		expect(session.principal).toBeNull();
		// A burst of 401s after the first: nothing left to expire.
		expect(session.expire()).toBe(false);
	});

	it('ignores a 401 for an anonymous visitor', () => {
		expect(new SessionStore().expire()).toBe(false);
	});
});
