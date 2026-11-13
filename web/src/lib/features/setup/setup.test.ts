import { afterEach, describe, expect, it, vi } from 'vitest';
import {
	adminLogin,
	clusterInputFromSetup,
	createSetupAdmin,
	createSetupCluster,
	fetchSetupStatus,
	isValidClusterName,
	probeCluster,
	setupStep,
	suggestClusterName,
	translateSetupError
} from './setup';
import type { SetupClusterInput, SetupStatus } from './setup';
import type { ClusterInput } from '#lib/features/admin-clusters/clusters.svelte.js';
import { ApiRequestError } from '#lib/shared/api/client.js';
import { m } from '#lib/paraglide/messages.js';

const fresh: SetupStatus = { required: true, needsAdmin: true, needsCluster: true };
const configured: SetupStatus = { required: false, needsAdmin: false, needsCluster: false };

const connection: SetupClusterInput = {
	url: 'https://pve.example:8006',
	tokenId: 'root@pam!pvmss',
	tokenSecret: 'secret',
	tlsInsecureSkipVerify: false
};

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

afterEach(() => vi.unstubAllGlobals());

describe('suggestClusterName', () => {
	it('passes an already valid name through', () => {
		expect(suggestClusterName('prod')).toBe('prod');
	});
	it('slugifies a display name', () => {
		expect(suggestClusterName('Prod Cluster')).toBe('prod-cluster');
	});
	it('strips characters outside [a-z0-9-]', () => {
		expect(suggestClusterName('PVE_01 (site A)')).toBe('pve-01-site-a');
	});
	it('collapses separators and trims dashes', () => {
		expect(suggestClusterName('--Prod--Cluster--')).toBe('prod-cluster');
	});
	it('returns empty string when nothing usable remains', () => {
		expect(suggestClusterName('!!!')).toBe('');
	});
});

describe('isValidClusterName', () => {
	it('accepts the server pattern', () => {
		expect(isValidClusterName('prod-01')).toBe(true);
	});
	it('rejects empty and invalid names', () => {
		expect(isValidClusterName('')).toBe(false);
		expect(isValidClusterName('Prod Cluster')).toBe(false);
		expect(isValidClusterName('pve_01')).toBe(false);
	});
});

describe('setupStep', () => {
	it('asks for an administrator account first', () => {
		expect(setupStep(fresh, false)).toBe('admin');
	});
	it('asks for sign-in when an admin exists but nobody is signed in', () => {
		expect(setupStep({ ...fresh, needsAdmin: false }, false)).toBe('signin');
	});
	it('asks for the first cluster once an admin is signed in', () => {
		expect(setupStep(fresh, true)).toBe('cluster');
		expect(setupStep({ ...fresh, needsAdmin: false }, true)).toBe('cluster');
	});
	it('keeps asking for an admin while the cluster is already set', () => {
		expect(setupStep({ ...fresh, needsCluster: false }, true)).toBe('admin');
	});
	it('is done when nothing is needed', () => {
		expect(setupStep(configured, true)).toBe('done');
		expect(setupStep(configured, false)).toBe('done');
	});
});

describe('translateSetupError', () => {
	it('maps a known server code to its translated copy', () => {
		const err = new ApiRequestError(400, 'invalid_username', 'username is invalid');
		expect(translateSetupError(err)).toBe(m['setup.error.invalidUsername']());
	});
	it('falls back to generic for unknown codes', () => {
		const err = new ApiRequestError(500, 'something_new', 'raw');
		expect(translateSetupError(err)).toBe(m['setup.error.generic']());
	});
	it('falls back to generic for non-API errors', () => {
		expect(translateSetupError(new Error('boom'))).toBe(m['setup.error.generic']());
		expect(translateSetupError('boom')).toBe(m['setup.error.generic']());
	});
});

describe('clusterInputFromSetup', () => {
	it('trims fields and ships an empty TLS block', () => {
		const input = clusterInputFromSetup('  prod  ', '  Prod Cluster  ', {
			...connection,
			url: '  https://pve.example:8006  ',
			tokenId: '  root@pam!pvmss  '
		});
		expect(input).toEqual({
			name: 'prod',
			displayName: 'Prod Cluster',
			url: 'https://pve.example:8006',
			tokenId: 'root@pam!pvmss',
			tokenSecret: 'secret',
			tlsInsecureSkipVerify: false,
			tls: {}
		});
	});
});

describe('setup API calls', () => {
	it('fetchSetupStatus GETs /api/v1/setup', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, fresh));
		vi.stubGlobal('fetch', fetchMock);

		const status = await fetchSetupStatus();

		expect(status).toEqual(fresh);
		const [path, init] = (fetchMock.mock.calls[0] ?? []) as [string, RequestInit | undefined];
		expect(path).toBe('/api/v1/setup');
		expect(init?.method ?? 'GET').toBe('GET');
	});

	it('createSetupAdmin POSTs the credentials', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { username: 'admin' }));
		vi.stubGlobal('fetch', fetchMock);

		const result = await createSetupAdmin({ username: 'admin', password: 'pw' });

		expect(result.username).toBe('admin');
		const [path, init] = (fetchMock.mock.calls[0] ?? []) as [string, RequestInit | undefined];
		expect(path).toBe('/api/v1/setup/admin');
		expect(init?.method).toBe('POST');
		expect(JSON.parse(String(init?.body))).toEqual({ username: 'admin', password: 'pw' });
	});

	it('adminLogin omits the username when blank', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { username: 'admin' }));
		vi.stubGlobal('fetch', fetchMock);

		await adminLogin('   ', 'pw');

		const [path, init] = (fetchMock.mock.calls[0] ?? []) as [string, RequestInit | undefined];
		expect(path).toBe('/api/v1/auth/admin-login');
		expect(JSON.parse(String(init?.body))).toEqual({ password: 'pw' });
	});

	it('adminLogin sends the trimmed username', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { username: 'bob' }));
		vi.stubGlobal('fetch', fetchMock);

		await adminLogin('  bob  ', 'pw');

		const [, init] = (fetchMock.mock.calls[0] ?? []) as [string, RequestInit | undefined];
		expect(JSON.parse(String(init?.body))).toEqual({ username: 'bob', password: 'pw' });
	});

	it('probeCluster POSTs the connection settings', async () => {
		const probe = { status: 'ok', clusterName: 'Prod', testedAt: '2026-11-13T00:00:00Z' };
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, probe));
		vi.stubGlobal('fetch', fetchMock);

		const result = await probeCluster(connection);

		expect(result.clusterName).toBe('Prod');
		const [path, init] = (fetchMock.mock.calls[0] ?? []) as [string, RequestInit | undefined];
		expect(path).toBe('/api/v1/admin/clusters/probe');
		expect(init?.method).toBe('POST');
		expect(JSON.parse(String(init?.body))).toEqual(connection);
	});

	it('createSetupCluster POSTs the cluster input', async () => {
		const input: ClusterInput = clusterInputFromSetup('prod', 'Prod', connection);
		const created = { name: 'prod', displayName: 'Prod' };
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, created));
		vi.stubGlobal('fetch', fetchMock);

		const result = await createSetupCluster(input);

		expect(result.name).toBe('prod');
		const [path, init] = (fetchMock.mock.calls[0] ?? []) as [string, RequestInit | undefined];
		expect(path).toBe('/api/v1/admin/clusters');
		expect(init?.method).toBe('POST');
		expect(JSON.parse(String(init?.body))).toEqual(input);
	});
});
