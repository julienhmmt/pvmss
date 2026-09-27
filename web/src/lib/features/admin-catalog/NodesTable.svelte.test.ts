import { beforeEach, describe, expect, it, vi } from 'vitest';
import { mount } from 'svelte';
import type { AdminNode } from './admin-catalog.svelte';
import NodesTable from './NodesTable.svelte';

vi.mock('$app/paths', () => ({
	resolve: (path: string, params?: Record<string, string>) =>
		Object.entries(params ?? {}).reduce((resolved, [key, value]) => resolved.replace(`[${key}]`, encodeURIComponent(value)), path)
}));

const node: AdminNode = {
	name: 'pve-node-01',
	status: 'online',
	cpuCores: 8,
	cpuUsage: 0.25,
	memoryTotal: 32000000000,
	memoryUsed: 8000000000,
	storageTotal: 1000000000000,
	storageUsed: 200000000000,
	vmCount: 5,
	enabled: true
};

beforeEach(() => {
	document.body.innerHTML = '';
});

describe('NodesTable', () => {
	it('links a discovered node name to its selected-cluster details', () => {
		mount(NodesTable, {
			target: document.body,
			props: {
				clusterKey: 'secondary',
				nodes: [node],
				toggling: null,
				sortBy: 'name',
				sortDir: 'asc',
				onToggle: vi.fn(),
				onRemove: vi.fn(),
				onSort: vi.fn()
			}
		});
		expect(document.querySelector<HTMLAnchorElement>('[data-testid="node-details-link"]')?.getAttribute('href')).toBe(
			'/admin/nodes/secondary/pve-node-01'
		);
		expect(document.querySelector('[role="switch"]')).not.toBeNull();
	});

	it('does not link an approval whose node is missing', () => {
		mount(NodesTable, {
			target: document.body,
			props: {
				clusterKey: 'secondary',
				nodes: [{ ...node, missing: true }],
				toggling: null,
				sortBy: 'name',
				sortDir: 'asc',
				onToggle: vi.fn(),
				onRemove: vi.fn(),
				onSort: vi.fn()
			}
		});
		expect(document.querySelector('[data-testid="node-details-link"]')).toBeNull();
		expect(document.querySelector('[data-testid="node-missing-badge"]')).not.toBeNull();
	});
});
