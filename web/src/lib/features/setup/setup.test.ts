import { describe, expect, it } from 'vitest';
import { isValidClusterName, setupStep, suggestClusterName } from './setup';
import type { SetupStatus } from './setup';

const fresh: SetupStatus = { required: true, needsAdmin: true, needsCluster: true };
const configured: SetupStatus = { required: false, needsAdmin: false, needsCluster: false };

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
	it('is done when nothing is needed', () => {
		expect(setupStep(configured, true)).toBe('done');
		expect(setupStep(configured, false)).toBe('done');
	});
});
