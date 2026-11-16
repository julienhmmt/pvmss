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
	tags: [{ name: 'oldtag', vmCount: 2, color: '#3b82f6' }],
	hiddenPools: [],
	hiddenLooseVMs: [],
	hiddenTags: []
};

const hiddenReport: RecoveryReport = {
	...report,
	hiddenPools: [{ name: 'pvmss-hidden', comment: 'dismissed', memberCount: 3 }],
	hiddenLooseVMs: [{ vmid: 910, name: 'ghost' }],
	hiddenTags: [{ name: 'ignored', vmCount: 4 }]
};

const emptyReport: RecoveryReport = {
	cluster: 'musclegrid',
	scannedAt: '2026-11-01T10:00:00Z',
	pools: [],
	looseVMs: [],
	tags: [],
	hiddenPools: [],
	hiddenLooseVMs: [],
	hiddenTags: []
};

function mountPage(overrides: Record<string, unknown> = {}): Record<string, ReturnType<typeof vi.fn>> {
	const effectiveReport = (overrides.report as RecoveryReport | undefined) ?? report;
	const callbacks = {
		onClusterChange: vi.fn(),
		onRescan: vi.fn(),
		onSearchChange: vi.fn(),
		onToggleAllPools: vi.fn(),
		onToggleAllTags: vi.fn(),
		onToggleAllVMs: vi.fn(),
		onTogglePool: vi.fn(),
		onToggleTag: vi.fn(),
		onToggleVM: vi.fn(),
		onAdopt: vi.fn(),
		onHideSelected: vi.fn(),
		onHide: vi.fn(),
		onUnhide: vi.fn(),
		onAdoptHidden: vi.fn()
	};
	mount(RecoveryPage, {
		target: document.body,
		props: {
			report,
			pools: effectiveReport.pools,
			tags: effectiveReport.tags,
			looseVMs: effectiveReport.looseVMs,
			search: '',
			allPoolsSelected: false,
			allTagsSelected: false,
			allVMsSelected: false,
			loading: false,
			rescanning: false,
			error: null,
			adopting: false,
			adoptError: null,
			hiding: false,
			hideError: null,
			announce: null,
			clusterOptions: [{ name: 'musclegrid', displayName: 'musclegrid' }],
			cluster: 'musclegrid',
			selectedPools: new Set(['pvmss-old']),
			selectedTags: new Set(['oldtag']),
			selectedVMs: new Set<number>(),
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
		const checkbox = [...document.querySelectorAll('input[type="checkbox"]')].find((el) =>
			el.closest('label')?.textContent?.includes('pvmss-old')
		);
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

	it('hides a pool from its per-row hide action', async () => {
		const callbacks = mountPage();
		const hideButton = document.querySelector('button[aria-label="Hide pvmss-old"]');
		hideButton?.dispatchEvent(new Event('click', { bubbles: true }));
		await tick();
		expect(callbacks.onHide).toHaveBeenCalledWith({ pools: ['pvmss-old'] });
	});

	it('hides a loose VM from its per-row hide action', async () => {
		const callbacks = mountPage();
		const hideButton = document.querySelector('button[aria-label="Hide solo-old"]');
		hideButton?.dispatchEvent(new Event('click', { bubbles: true }));
		await tick();
		expect(callbacks.onHide).toHaveBeenCalledWith({ vms: [902] });
	});

	it('toggles a loose VM hide selection from its checkbox', async () => {
		const callbacks = mountPage();
		const checkbox = document.querySelector('input[aria-label="Select solo-old"]');
		checkbox?.dispatchEvent(new Event('change', { bubbles: true }));
		await tick();
		expect(callbacks.onToggleVM).toHaveBeenCalledWith(902);
	});

	it('lists hidden items in the collapsed section with unhide and adopt', async () => {
		const callbacks = mountPage({ report: hiddenReport });
		expect(text()).toContain('Hidden (3 items)');
		expect(text()).toContain('pvmss-hidden');
		expect(text()).toContain('ghost');
		expect(text()).toContain('ignored');

		const unhideButton = document.querySelector('button[aria-label="Show pvmss-hidden again"]');
		unhideButton?.dispatchEvent(new Event('click', { bubbles: true }));
		await tick();
		expect(callbacks.onUnhide).toHaveBeenCalledWith({ pools: ['pvmss-hidden'] });
	});

	it('adopts a hidden tag directly from the hidden section', async () => {
		const callbacks = mountPage({ report: hiddenReport });
		const adoptButtons = [...document.querySelectorAll('button')].filter(
			(candidate) => candidate.textContent?.trim() === 'Adopt'
		);
		// hidden pools first, then hidden tags: the second Adopt is the tag's.
		adoptButtons[1]?.click();
		await tick();
		expect(callbacks.onAdoptHidden).toHaveBeenCalledWith({ tag: 'ignored' });
	});

	it('forwards search input to onSearchChange', async () => {
		const callbacks = mountPage();
		const input = document.querySelector('input[type="search"]');
		input?.dispatchEvent(new Event('input', { bubbles: true }));
		await tick();
		expect(callbacks.onSearchChange).toHaveBeenCalled();
	});

	it('shows the no-match state when the filter empties every list', () => {
		mountPage({ pools: [], tags: [], looseVMs: [], search: 'zzz' });
		expect(text()).toContain('No resources match "zzz"');
	});

	it('toggles select-all on the pools card', async () => {
		const callbacks = mountPage();
		const selectAll = [...document.querySelectorAll('input[type="checkbox"]')].find((el) =>
			el.closest('div')?.textContent?.includes('Select all')
		);
		selectAll?.dispatchEvent(new Event('change', { bubbles: true }));
		await tick();
		expect(callbacks.onToggleAllPools).toHaveBeenCalled();
	});
});
