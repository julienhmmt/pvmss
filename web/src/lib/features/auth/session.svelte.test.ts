import { describe, expect, it } from 'vitest';
import { SessionStore } from './session.svelte';

const principal = {
	username: 'alice@pve',
	displayName: 'alice',
	pool: 'pool-alice',
	isAdmin: false,
	cluster: 'default',
	clusterDisplayName: 'Default',
	provider: 'pve' as const,
	isOwner: false
};

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

	it('exposes admin and owner flags from the principal', () => {
		const session = new SessionStore();
		expect(session.isAdmin).toBe(false);
		expect(session.isOwner).toBe(false);

		session.principal = { ...principal, isAdmin: true, provider: 'bootstrap', isOwner: true };
		expect(session.isAdmin).toBe(true);
		expect(session.isOwner).toBe(true);
	});
});
