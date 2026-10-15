import { describe, expect, it } from 'vitest';
import { CAPABILITIES, type CapabilityId } from './capability-data';

describe('CAPABILITIES', () => {
	it('exports one entry per capability id', () => {
		const ids = CAPABILITIES.map((c) => c.id);
		expect(ids).toEqual<CapabilityId[]>([
			'lifecycle',
			'consoles',
			'cloudinit',
			'snapshots',
			'storage',
			'multiCluster'
		]);
	});

	it('every capability has a non-empty title and description', () => {
		for (const capability of CAPABILITIES) {
			expect(typeof capability.title()).toBe('string');
			expect(capability.title().length).toBeGreaterThan(0);
			expect(typeof capability.description()).toBe('string');
			expect(capability.description().length).toBeGreaterThan(0);
		}
	});

	it('ids are unique', () => {
		const ids = CAPABILITIES.map((c) => c.id);
		expect(new Set(ids).size).toBe(ids.length);
	});
});
