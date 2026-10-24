import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount } from 'svelte';
import type { AdminBridge } from './admin-catalog.svelte';
import BridgesTable from './BridgesTable.svelte';
import { setLocale } from '#lib/paraglide/runtime.js';

const bridge = (node: string): AdminBridge => ({ name: 'vmbr0', node, active: true, comment: '', enabled: false, missing: false });

beforeAll(() => setLocale('en', { reload: false }));
beforeEach(() => {
	document.body.innerHTML = '';
});

describe('BridgesTable', () => {
	it('names the node in each switch label, so same-named bridges stay distinct', () => {
		mount(BridgesTable, {
			target: document.body,
			props: {
				bridges: [bridge('pve-node-01'), bridge('pve-node-02')],
				toggling: null,
				sortBy: 'name',
				sortDir: 'asc',
				onToggle: vi.fn(),
				onRemove: vi.fn(),
				onSort: vi.fn()
			}
		});
		const labels = [...document.querySelectorAll('[role="switch"]')].map((el) => el.getAttribute('aria-label'));
		expect(labels).toEqual(['Enable vmbr0 (pve-node-01)', 'Enable vmbr0 (pve-node-02)']);
	});
});
