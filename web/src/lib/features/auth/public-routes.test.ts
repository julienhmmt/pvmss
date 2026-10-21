import { describe, it, expect } from 'vitest';
import { isPublicPath, loginHref, safeNext } from './public-routes';

describe('isPublicPath', () => {
	it('returns true for exact public paths', () => {
		expect(isPublicPath('/')).toBe(true);
		expect(isPublicPath('/login')).toBe(true);
	});

	it('returns true for /docs and its children', () => {
		expect(isPublicPath('/docs')).toBe(true);
		expect(isPublicPath('/docs/getting-started')).toBe(true);
		expect(isPublicPath('/docs/admin/policy')).toBe(true);
	});

	it('returns false for protected paths', () => {
		expect(isPublicPath('/vms')).toBe(false);
		expect(isPublicPath('/nodes')).toBe(false);
		expect(isPublicPath('/admin/nodes')).toBe(false);
	});

	it('returns false for /docs-like prefixes that are not /docs', () => {
		expect(isPublicPath('/docs-private')).toBe(false);
		expect(isPublicPath('/documentation')).toBe(false);
	});
});

describe('safeNext', () => {
	it('keeps an in-app path with its query', () => {
		expect(safeNext('/vms?cluster=default')).toBe('/vms?cluster=default');
	});

	it('rejects missing, external and self-referencing targets', () => {
		for (const raw of [null, '', 'vms', '//evil.example', '/\\evil.example', '/\t/evil.example', 'https://evil.example/', '/login', '/login?next=/vms']) {
			expect(safeNext(raw)).toBeNull();
		}
	});
});

describe('loginHref', () => {
	it('carries the current path as an encoded next', () => {
		expect(loginHref('/vms?cluster=a&page=2')).toBe('/login?next=%2Fvms%3Fcluster%3Da%26page%3D2');
	});
});
