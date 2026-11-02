import { beforeAll, afterEach, describe, expect, it, vi } from 'vitest';
import { mount, tick } from 'svelte';
import VmHardwareTab from './VmHardwareTab.svelte';
import { getVmDetailContext, type VmDetailStore } from '../detail.svelte';
import { setLocale } from '#lib/paraglide/runtime.js';

vi.mock('../detail.svelte', async (importOriginal) => {
	const mod = await importOriginal<typeof import('../detail.svelte')>();
	return { ...mod, getVmDetailContext: vi.fn() };
});

function render(overrides: Record<string, unknown>): void {
	const store = {
		entity: {
			name: 'vm',
			status: 'stopped',
			sockets: 1,
			cores: 2,
			cpuCores: 2,
			memoryTotal: 1024 * 1024 * 1024,
			tags: []
		},
		hardwareOptions: null,
		hardwareLoading: false,
		hardwareError: null,
		hardwareInFlight: false,
		writeError: null,
		updateHardware: vi.fn(),
		loadHardwareOptions: vi.fn(),
		...overrides
	} as unknown as VmDetailStore;
	vi.mocked(getVmDetailContext).mockReturnValue(store);
	mount(VmHardwareTab, { target: document.body });
}

const q = (id: string) => document.querySelector(`[data-testid="${id}"]`);
const findButton = (label: string) => [...document.querySelectorAll('button')].find((b) => b.textContent?.trim() === label);

describe('VmHardwareTab load states', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('shows an alert and no editable inputs or save button when the options fetch failed', async () => {
		render({ hardwareError: 'nope' });
		await tick();

		expect(document.body.textContent).toContain('nope');
		expect(q('vm-hardware-sockets')).toBeNull();
		expect(q('vm-hardware-cores')).toBeNull();
		expect(q('vm-hardware-memory')).toBeNull();
		expect(q('vm-hardware-save')).toBeNull();
	});

	it('offers a retry that reloads the hardware options', async () => {
		const loadHardwareOptions = vi.fn();
		render({ hardwareError: 'nope', loadHardwareOptions });
		await tick();

		findButton('Retry')?.click();
		expect(loadHardwareOptions).toHaveBeenCalledTimes(1);
	});

	it('shows the form when the options loaded', async () => {
		render({ hardwareOptions: { limits: {}, tags: [] } });
		await tick();

		expect(q('vm-hardware-sockets')).not.toBeNull();
		expect(q('vm-hardware-save')).not.toBeNull();
	});
});
