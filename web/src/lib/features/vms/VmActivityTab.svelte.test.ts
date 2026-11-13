import { beforeAll, afterEach, describe, expect, it, vi } from 'vitest';
import { mount, tick } from 'svelte';
import VmActivityTab from './VmActivityTab.svelte';
import { getVmDetailContext, type VmAuditEntry, type VmDetailStore } from './detail.svelte';
import { setLocale } from '#lib/paraglide/runtime.js';

vi.mock('./detail.svelte', async (importOriginal) => {
	const mod = await importOriginal<typeof import('./detail.svelte')>();
	return { ...mod, getVmDetailContext: vi.fn() };
});

function render(overrides: Record<string, unknown>): VmDetailStore {
	const store = {
		vmid: 100,
		auditItems: null,
		auditLoading: false,
		auditError: null,
		auditPage: 1,
		auditPageSize: 10,
		auditTotal: 20,
		audit: vi.fn(),
		auditAll: vi.fn((): Promise<VmAuditEntry[]> => Promise.resolve([])),
		...overrides
	} as unknown as VmDetailStore;
	vi.mocked(getVmDetailContext).mockReturnValue(store);
	mount(VmActivityTab, { target: document.body });
	return store;
}

const entry = { id: 1, actor: 'alice', action: 'vm.start', timestamp: '2026-01-01T00:00:00Z' };
const entry2 = { id: 2, actor: 'bob', action: 'vm.stop', timestamp: '2026-01-02T00:00:00Z' };

function stubDownload(): { createObjectURL: ReturnType<typeof vi.fn> } {
	const createObjectURL = vi.fn((): string => 'blob:mock');
	URL.createObjectURL = createObjectURL;
	URL.revokeObjectURL = vi.fn();
	vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
	return { createObjectURL };
}

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

describe('VmActivityTab export', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
		vi.restoreAllMocks();
	});

	it('selects the whole page and reflects the count on the export button', async () => {
		render({ auditItems: [entry, entry2] });
		await tick();

		const selectAll = document.querySelector<HTMLInputElement>('[data-testid="vm-activity-select-all"]')!;
		selectAll.click();
		await tick();

		const button = document.querySelector('[data-testid="vm-activity-export"]')!;
		expect(button.textContent).toContain('Export selection (2)');
		const rowBoxes = document.querySelectorAll<HTMLInputElement>('[data-testid="vm-activity-select-row"]');
		expect([...rowBoxes].every((box) => box.checked)).toBe(true);

		selectAll.click();
		await tick();
		expect(button.textContent).toContain('Export CSV');
	});

	it('exports the selection without calling auditAll', async () => {
		const auditAll = vi.fn((): Promise<VmAuditEntry[]> => Promise.resolve([]));
		render({ auditItems: [entry, entry2], auditAll });
		await tick();
		const { createObjectURL } = stubDownload();

		document.querySelector<HTMLInputElement>('[data-testid="vm-activity-select-row"]')!.click();
		await tick();
		document.querySelector<HTMLElement>('[data-testid="vm-activity-export"]')!.click();
		await tick();

		expect(auditAll).not.toHaveBeenCalled();
		expect(createObjectURL).toHaveBeenCalled();
	});

	it('walks pages via auditAll when nothing is selected', async () => {
		const auditAll = vi.fn((): Promise<VmAuditEntry[]> => Promise.resolve([]));
		render({ auditItems: [entry], auditAll });
		await tick();
		const { createObjectURL } = stubDownload();

		document.querySelector<HTMLElement>('[data-testid="vm-activity-export"]')!.click();
		await tick();

		expect(auditAll).toHaveBeenCalledWith(undefined);
		expect(createObjectURL).toHaveBeenCalled();
	});

	it('shows an alert when the export fails', async () => {
		render({
			auditItems: [entry],
			auditAll: vi.fn((): Promise<VmAuditEntry[]> => Promise.reject(new Error('boom')))
		});
		await tick();
		stubDownload();

		document.querySelector<HTMLElement>('[data-testid="vm-activity-export"]')!.click();
		await tick();
		await tick();

		expect(document.querySelector('[data-testid="vm-activity-export-error"]')?.textContent).toContain(
			'Failed to export activity.'
		);
	});
});
