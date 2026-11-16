import { beforeEach, describe, expect, it } from 'vitest';
import { orderPinnedFirst, readPins, togglePin } from './pinned-vms';

const vm = (vmid: number, cluster = 'a') => ({ cluster, vmid });

describe('pinned VMs', () => {
	beforeEach(() => localStorage.clear());

	it('persists pins per user and toggles them off', () => {
		const pinned = togglePin('alice', new Set(), 'a/100');
		expect(readPins('alice').has('a/100')).toBe(true);
		expect(readPins('bob').size).toBe(0);
		togglePin('alice', pinned, 'a/100');
		expect(readPins('alice').size).toBe(0);
	});

	it('does not mutate the set it is given', () => {
		const before = new Set(['a/1']);
		togglePin('alice', before, 'a/2');
		expect([...before]).toEqual(['a/1']);
	});

	it('reads junk storage as nothing pinned', () => {
		localStorage.setItem('pvmss-pinned-vms-v1:alice', '{nope');
		expect(readPins('alice').size).toBe(0);
		localStorage.setItem('pvmss-pinned-vms-v1:alice', '[1, "a/2"]');
		expect([...readPins('alice')]).toEqual(['a/2']);
	});

	it('puts pinned machines first and caps the others', () => {
		const list = [vm(1), vm(2), vm(3), vm(4, 'b')];
		const ordered = orderPinnedFirst(list, new Set(['b/4', 'a/3']), 1);
		expect(ordered.map((v) => v.vmid)).toEqual([3, 4, 1]);
	});
});
