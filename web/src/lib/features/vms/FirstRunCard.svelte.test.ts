import { describe, it, expect, beforeAll, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushSync } from 'svelte';
import FirstRunCard from './FirstRunCard.svelte';
import { setLocale } from '#lib/paraglide/runtime.js';

const api = vi.hoisted(() => ({ catalog: { profiles: [{}], templates: [] }, clusters: [{ name: 'default' }] as { name: string }[] }));
vi.mock('#lib/shared/api/client.js', async (importOriginal) => {
	const mod = await importOriginal<typeof import('#lib/shared/api/client.js')>();
	return {
		...mod,
		// QuotaRequestPanel reads the user's latest request; none is open.
		get: vi.fn((path: string) => Promise.resolve(path.includes('quota-requests') ? null : api.catalog))
	};
});
vi.mock('#lib/shared/clusters.js', () => ({ fetchClusterOptions: vi.fn(() => Promise.resolve(api.clusters)) }));
vi.mock('#lib/features/quota-requests/quota-requests', () => ({
	fetchMyQuotaRequest: vi.fn(() => Promise.resolve(null)),
	submitQuotaRequest: vi.fn()
}));
const session = vi.hoisted(() => ({ isAdmin: false, principal: { cluster: 'paris', clusterDisplayName: 'Paris' } }));
vi.mock('#lib/features/auth/session.svelte.js', () => ({ getSessionContext: vi.fn(() => session) }));

const byId = (id: string): Element | null => document.querySelector(`[data-testid="${id}"]`);
async function settle(): Promise<void> {
	await new Promise((resolve) => setTimeout(resolve, 0));
	flushSync();
}

describe('FirstRunCard', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	beforeEach(() => {
		api.catalog = { profiles: [{}], templates: [] };
		api.clusters = [{ name: 'default' }];
		session.isAdmin = false;
	});
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('explains the allowance and links the create wizard and the guide', async () => {
		mount(FirstRunCard, { target: document.body, props: { quota: { used: 0, allowed: 3 } } });
		await settle();
		expect(byId('vm-first-run-allowance')?.textContent).toContain('You can run up to 3 machines.');
		expect(byId('vm-empty-create')).not.toBeNull();
		expect(byId('vm-first-run-guide')?.getAttribute('href')).toContain('/docs/getting-started');
	});

	it('names the cluster when more than one exists', async () => {
		api.clusters = [{ name: 'paris' }, { name: 'lyon' }];
		mount(FirstRunCard, { target: document.body, props: { quota: { used: 0, allowed: 1 } } });
		await settle();
		expect(byId('vm-first-run-allowance')?.textContent).toContain('You can run up to 1 machine on Paris.');
	});

	it('says there is no limit for an unlimited allowance', async () => {
		mount(FirstRunCard, { target: document.body, props: { quota: { used: 0, allowed: -1 } } });
		await settle();
		expect(byId('vm-first-run-allowance')?.textContent).toContain('no machine limit');
	});

	it('omits the allowance line when the list carries no quota', async () => {
		mount(FirstRunCard, { target: document.body, props: { quota: undefined } });
		await settle();
		expect(byId('vm-first-run-allowance')).toBeNull();
		expect(byId('vm-empty-create')).not.toBeNull();
	});

	it('offers a quota request instead of a create on zero allowance', async () => {
		mount(FirstRunCard, { target: document.body, props: { quota: { used: 0, allowed: 0 } } });
		await settle();
		expect(byId('vm-first-run-zero')).not.toBeNull();
		expect(byId('vm-empty-create')).toBeNull();
	});

	it('drops the create invite when no size or template is approved', async () => {
		api.catalog = { profiles: [], templates: [] };
		mount(FirstRunCard, { target: document.body, props: { quota: { used: 0, allowed: 3 } } });
		await settle();
		expect(byId('vm-first-run-no-catalog')).not.toBeNull();
		expect(byId('vm-empty-create')).toBeNull();
	});

	it('keeps the admin variant without allowance or create', async () => {
		session.isAdmin = true;
		mount(FirstRunCard, { target: document.body, props: { quota: { used: 0, allowed: 3 } } });
		await settle();
		expect(byId('vm-empty-owned')).not.toBeNull();
		expect(byId('vm-empty-create')).toBeNull();
		expect(byId('vm-first-run-allowance')).toBeNull();
	});
});
