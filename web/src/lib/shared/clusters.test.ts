import { afterEach, describe, expect, it, vi } from 'vitest';
import {
	getSavedAdminCluster,
	pinnedClusterOptions,
	resolveInitialAdminCluster,
	saveAdminCluster,
	type ClusterOption
} from './clusters';
import { AdminCatalogStore } from '#lib/features/admin-catalog/admin-catalog.svelte.js';
import { AdminPolicyStore } from '#lib/features/admin-policy/policy.svelte.js';
import { AdminBaselineStore } from '#lib/features/admin-baseline/baseline.svelte.js';

const options: ClusterOption[] = [
	{ name: 'default', displayName: 'Default' },
	{ name: 'secondary', displayName: 'Secondary' }
];

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('Admin active cluster persistence (Ticket 12)', () => {
	afterEach(() => {
		sessionStorage.clear();
		vi.unstubAllGlobals();
	});

	it('saves and restores the selected admin cluster in sessionStorage', () => {
		expect(getSavedAdminCluster()).toBeNull();
		saveAdminCluster('secondary');
		expect(getSavedAdminCluster()).toBe('secondary');
		expect(resolveInitialAdminCluster(options, 'default')).toBe('secondary');
	});

	it('prefers an explicit URL ?cluster= hint and updates sessionStorage', () => {
		saveAdminCluster('secondary');
		expect(resolveInitialAdminCluster(options, 'secondary', 'default')).toBe('default');
		expect(getSavedAdminCluster()).toBe('default');
	});

	it('falls back to the first available cluster when the saved cluster no longer exists', () => {
		saveAdminCluster('deleted-cluster');
		expect(resolveInitialAdminCluster(options, 'default')).toBe('default');
	});

	it('carries the selected cluster from AdminCatalogStore to AdminPolicyStore and AdminBaselineStore', async () => {
		const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL) => {
			const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
			if (url.includes('/api/v1/auth/clusters')) {
				return Promise.resolve(jsonResponse(200, options));
			}
			if (url.includes('/api/v1/admin/policy')) {
				return Promise.resolve(
					jsonResponse(200, {
						cluster: 'secondary',
						gabarit: {
							maxSockets: 2,
							maxCores: 8,
							maxMemoryMB: 16384,
							maxDiskPerVmGb: 100,
							maxNetworkCards: 4,
							maxSnapshots: 5,
							isolationVlanTag: 0
						},
						quota: { maxVmPerUser: 5 }
					})
				);
			}
			if (url.includes('/api/v1/admin/baseline')) {
				return Promise.resolve(jsonResponse(200, { generated: '#cloud-config\n', document: null }));
			}
			return Promise.resolve(jsonResponse(200, []));
		});
		vi.stubGlobal('fetch', fetchMock);

		const catalog = new AdminCatalogStore();
		catalog.setCluster('secondary');

		const policy = new AdminPolicyStore();
		await policy.load();
		expect(policy.cluster).toBe('secondary');

		const baseline = new AdminBaselineStore();
		await baseline.init();
		expect(baseline.cluster).toBe('secondary');
	});
});

describe('pinnedClusterOptions', () => {
	const options: ClusterOption[] = [
		{ name: 'a', displayName: 'A' },
		{ name: 'b', displayName: 'B' }
	];

	it('keeps only the sign-in cluster for a non-admin', () => {
		expect(pinnedClusterOptions(options, { isAdmin: false, cluster: 'a' })).toEqual([options[0]]);
	});

	it('keeps every cluster for an admin, a legacy session or no session', () => {
		expect(pinnedClusterOptions(options, { isAdmin: true, cluster: 'a' })).toEqual(options);
		expect(pinnedClusterOptions(options, { isAdmin: false, cluster: '' })).toEqual(options);
		expect(pinnedClusterOptions(options, null)).toEqual(options);
	});
});
