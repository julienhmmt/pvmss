import { describe, it, expect } from 'vitest';
import { isPublicPath } from './public-routes';

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
