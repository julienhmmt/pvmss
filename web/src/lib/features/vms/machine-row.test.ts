import { beforeAll, describe, expect, it } from 'vitest';
import { setLocale } from '$lib/paraglide/runtime.js';
import { compactBytes, rowAction, rowHint } from './machine-row';
import type { MachineDisplayStatus } from './display-status';

beforeAll(() => setLocale('en', { reload: false }));

describe('rowHint and rowAction', () => {
	const statuses: MachineDisplayStatus[] = ['running', 'stopped', 'provisioning', 'starting', 'stopping', 'failed', 'partial'];

	it('explains every state but running, with failed and partial as errors', () => {
		for (const status of statuses) {
			const hint = rowHint(status);
			if (status === 'running') {
				expect(hint).toBeNull();
				continue;
			}
			expect(hint?.text.length).toBeGreaterThan(0);
			expect(hint?.tone).toBe(status === 'failed' || status === 'partial' ? 'error' : 'muted');
		}
		expect(rowHint('partial')?.text).toContain('Do not create a duplicate');
	});

	it('offers Start only on a stopped machine and never Connect', () => {
		for (const status of statuses) {
			expect(rowAction(status)).toBe(status === 'stopped' ? 'start' : 'details');
		}
	});
});

describe('compactBytes', () => {
	it.each([
		[8 * 1024 ** 3, '8 GiB'],
		[1.5 * 1024 ** 3, '1.5 GiB'],
		[512 * 1024 ** 2, '512 MiB']
	])('%d -> %s', (bytes, text) => {
		expect(compactBytes(bytes)).toBe(text);
	});
});
