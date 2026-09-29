import { describe, it, expect, beforeAll, afterEach, vi } from 'vitest';
import { mount, tick } from 'svelte';
import VmCdromCard from './VmCdromCard.svelte';
import { getVmDetailContext, type VmCdrom, type VmDetailStore } from '../detail.svelte';
import { getToastContext } from '$lib/shared/ui/toast.svelte';
import { setLocale } from '$lib/paraglide/runtime.js';

vi.mock('../detail.svelte', async (importOriginal) => {
	const mod = await importOriginal<typeof import('../detail.svelte')>();
	return { ...mod, getVmDetailContext: vi.fn() };
});
vi.mock('$lib/shared/ui/toast.svelte', () => ({ getToastContext: vi.fn() }));

async function render(cdrom: VmCdrom): Promise<void> {
	const store = { entity: { name: 'vm', status: 'running', cdrom }, cdromInFlight: false } as unknown as VmDetailStore;
	vi.mocked(getVmDetailContext).mockReturnValue(store);
	vi.mocked(getToastContext).mockReturnValue({} as ReturnType<typeof getToastContext>);
	mount(VmCdromCard, { target: document.body });
	await tick();
}

const button = (id: string) => document.querySelector<HTMLButtonElement>(`[data-testid="${id}"]`);

describe('VmCdromCard', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('locks the drive controls when ide2 holds a cloud-init drive', async () => {
		await render({ state: 'occupied' });

		expect(document.querySelector('[data-testid="vm-cdrom-occupied"]')).not.toBeNull();
		for (const id of ['vm-cdrom-mount-open', 'vm-cdrom-disconnect', 'vm-cdrom-remove']) {
			expect(button(id)?.disabled).toBe(true);
		}
	});

	it('keeps the controls enabled for an empty drive', async () => {
		await render({ state: 'empty' });

		expect(document.querySelector('[data-testid="vm-cdrom-occupied"]')).toBeNull();
		expect(button('vm-cdrom-mount-open')?.disabled).toBe(false);
	});
});
