import { describe, it, expect, beforeAll, afterEach, vi } from 'vitest';
import { mount, tick } from 'svelte';
import RecoveryPage from './RecoveryPage.svelte';
import type { RecoveryReport } from './recovery.svelte';
import { setLocale } from '#lib/paraglide/runtime.js';

const report: RecoveryReport = {
	cluster: 'musclegrid',
	scannedAt: '2026-11-01T10:00:00Z',
	pools: [
		{
			name: 'pvmss-old',
			comment: 'previous install',
			members: [
				{ vmid: 900, name: 'web-old-01', node: 'n1', status: 'running', pool: 'pvmss-old', tags: ['pvmss', 'oldtag'] },
				{ vmid: 901, name: 'web-old-02', node: 'n1', status: 'stopped', pool: 'pvmss-old', tags: ['pvmss'] }
			]
		}
	],
	looseVMs: [{ vmid: 902, name: 'solo-old', node: 'n2', status: 'stopped', tags: ['pvmss', 'oldtag'] }],
	tags: [{ name: 'oldtag', vmCount: 2, color: '#3b82f6' }]
};

const emptyReport: RecoveryReport = { cluster: 'musclegrid', scannedAt: '2026-11-01T10:00:00Z', pools: [], looseVMs: [], tags: [] };

function mountPage(overrides: Record<string, unknown> = {}): Record<string, ReturnType<typeof vi.fn>> {
	const callbacks = {
		onClusterChange: vi.fn(),
		onRescan: vi.fn(),
		onTogglePool: vi.fn(),
		onToggleTag: vi.fn(),
		onAdopt: vi.fn()
	};
	mount(RecoveryPage, {
		target: document.body,
		props: {
			report,
			loading: false,
			rescanning: false,
			error: null,
			adopting: false,
			adoptError: null,
			announce: null,
			clusterOptions: [{ name: 'musclegrid', displayName: 'musclegrid' }],
			cluster: 'musclegrid',
			selectedPools: new Set(['pvmss-old']),
			selectedTags: new Set(['oldtag']),
			...callbacks,
			...overrides
		}
	});
	return callbacks;
}

function text(): string {
	return document.body.textContent ?? '';
}

function buttonWith(label: string): HTMLButtonElement | undefined {
	return [...document.querySelectorAll('button')].find((candidate) => candidate.textContent?.includes(label));
}

describe('RecoveryPage', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
		vi.restoreAllMocks();
	});

	it('lists orphan pools, member and loose VMs, and unknown tags', () => {
		mountPage();
		expect(text()).toContain('pvmss-old');
		expect(text()).toContain('web-old-01');
		expect(text()).toContain('web-old-02');
		expect(text()).toContain('solo-old');
		expect(text()).toContain('oldtag');
	});

	it('toggles a pool selection from its checkbox', async () => {
		const callbacks = mountPage();
		const checkbox = document.querySelector('input[type="checkbox"]');
		checkbox?.dispatchEvent(new Event('change', { bubbles: true }));
		await tick();
		expect(callbacks.onTogglePool).toHaveBeenCalledWith('pvmss-old');
	});

	it('submits adoption from the adopt button', async () => {
		const callbacks = mountPage();
		buttonWith('Adopt selected')?.click();
		await tick();
		expect(callbacks.onAdopt).toHaveBeenCalled();
	});

	it('forces a rescan from the rescan button', async () => {
		const callbacks = mountPage();
		buttonWith('Rescan')?.click();
		await tick();
		expect(callbacks.onRescan).toHaveBeenCalled();
	});

	it('shows the empty state when nothing is left', () => {
		mountPage({ report: emptyReport });
		expect(text()).toContain('Nothing left from a previous install');
	});
});
