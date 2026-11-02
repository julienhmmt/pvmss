import { beforeAll, afterEach, describe, expect, it, vi } from 'vitest';
import { mount, tick } from 'svelte';
import VmActivityTab from './VmActivityTab.svelte';
import { getVmDetailContext, type VmDetailStore } from './detail.svelte';
import { setLocale } from '#lib/paraglide/runtime.js';

vi.mock('./detail.svelte', async (importOriginal) => {
	const mod = await importOriginal<typeof import('./detail.svelte')>();
	return { ...mod, getVmDetailContext: vi.fn() };
});

function render(overrides: Record<string, unknown>): void {
	const store = {
		auditItems: null,
		auditLoading: false,
		auditError: null,
		auditPage: 1,
		auditPageSize: 10,
		auditTotal: 20,
		audit: vi.fn(),
		...overrides
	} as unknown as VmDetailStore;
	vi.mocked(getVmDetailContext).mockReturnValue(store);
	mount(VmActivityTab, { target: document.body });
}

const entry = { id: 1, actor: 'alice', action: 'vm.start', timestamp: '2026-01-01T00:00:00Z' };

describe('VmActivityTab errors', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('keeps the rows and shows the error when a later page fails', async () => {
		render({ auditItems: [entry], auditError: 'page failed' });
		await tick();

		expect(document.querySelector('[data-testid="vm-activity-table"]')).not.toBeNull();
		expect(document.querySelector('[data-testid="vm-activity-page-error"]')?.textContent).toContain('page failed');
	});

	it('shows the full error branch when nothing loaded', async () => {
		render({ auditItems: null, auditError: 'first failed' });
		await tick();

		expect(document.querySelector('[data-testid="vm-activity-error"]')?.textContent).toContain('first failed');
		expect(document.querySelector('[data-testid="vm-activity-page-error"]')).toBeNull();
	});
});
