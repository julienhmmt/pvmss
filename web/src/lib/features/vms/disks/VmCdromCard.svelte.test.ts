import { describe, it, expect, beforeAll, afterEach, vi } from 'vitest';
import { mount, tick } from 'svelte';
import VmCdromCard from './VmCdromCard.svelte';
import { getVmDetailContext, type VmCdrom, type VmDetailStore } from '../detail.svelte';
import { getToastContext } from '#lib/shared/ui/toast.svelte.js';
import { setLocale } from '#lib/paraglide/runtime.js';

vi.mock('../detail.svelte', async (importOriginal) => {
	const mod = await importOriginal<typeof import('../detail.svelte')>();
	return { ...mod, getVmDetailContext: vi.fn() };
});
vi.mock('#lib/shared/ui/toast.svelte.js', () => ({ getToastContext: vi.fn() }));

async function render(cdrom: VmCdrom): Promise<{ setCdrom: ReturnType<typeof vi.fn> }> {
	const setCdrom = vi.fn();
	const store = { entity: { name: 'vm', status: 'running', cdrom }, cdromInFlight: false, setCdrom } as unknown as VmDetailStore;
	vi.mocked(getVmDetailContext).mockReturnValue(store);
	vi.mocked(getToastContext).mockReturnValue({} as ReturnType<typeof getToastContext>);
	mount(VmCdromCard, { target: document.body });
	await tick();
	return { setCdrom };
}

const confirmDialogButton = (id: string) => document.querySelector<HTMLButtonElement>(`[data-testid="${id}"]`);

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

	it.each([
		['absent', 'No drive'],
		['empty', 'Empty'],
		['mounted', 'ISO mounted'],
		['occupied', 'Cloud-init drive']
	] as const)('labels the %s state instead of printing the enum', async (state, label) => {
		await render({ state });

		expect(document.querySelector('[data-testid="vm-cdrom-state"]')?.textContent).toContain(label);
	});

	it('falls back to the raw value for a state it does not know', async () => {
		await render({ state: 'ejected' as VmCdrom['state'] });

		expect(document.querySelector('[data-testid="vm-cdrom-state"]')?.textContent).toContain('ejected');
	});

	it('asks before removing the drive and removes only on confirm', async () => {
		const { setCdrom } = await render({ state: 'mounted' });

		button('vm-cdrom-remove')?.click();
		await tick();
		expect(setCdrom).not.toHaveBeenCalled();
		expect(confirmDialogButton('vm-cdrom-remove-confirm')).not.toBeNull();

		confirmDialogButton('vm-cdrom-remove-confirm')?.click();
		await tick();
		expect(setCdrom).toHaveBeenCalledTimes(1);
		expect(setCdrom).toHaveBeenCalledWith('remove');
	});

	it('does not remove the drive when the removal is cancelled', async () => {
		const { setCdrom } = await render({ state: 'mounted' });

		button('vm-cdrom-remove')?.click();
		await tick();
		const cancel = [...document.querySelectorAll<HTMLButtonElement>('button')].find(
			(b) => b.textContent?.trim() === 'Cancel'
		);
		cancel?.click();
		await tick();
		expect(setCdrom).not.toHaveBeenCalled();
	});

	it('keeps the controls enabled for an empty drive', async () => {
		await render({ state: 'empty' });

		expect(document.querySelector('[data-testid="vm-cdrom-occupied"]')).toBeNull();
		expect(button('vm-cdrom-mount-open')?.disabled).toBe(false);
	});
});
