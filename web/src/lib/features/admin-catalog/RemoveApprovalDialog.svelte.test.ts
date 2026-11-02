import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { mount, tick } from 'svelte';
import RemoveApprovalDialog from './RemoveApprovalDialog.svelte';
import { setLocale } from '#lib/paraglide/runtime.js';

const buttonByText = (label: string): HTMLButtonElement | undefined =>
	[...document.querySelectorAll<HTMLButtonElement>('button')].find((b) => b.textContent?.trim() === label);

describe('RemoveApprovalDialog', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('names the item in the title and calls onConfirm only from the remove button', async () => {
		const onConfirm = vi.fn();
		const onClose = vi.fn();
		mount(RemoveApprovalDialog, { target: document.body, props: { open: true, name: 'pve-1', onConfirm, onClose } });
		await tick();

		expect(document.body.textContent).toContain('Forget pve-1?');
		expect(onConfirm).not.toHaveBeenCalled();

		buttonByText('Remove')?.click();
		await tick();
		expect(onConfirm).toHaveBeenCalledTimes(1);
	});

	it('calls onClose on cancel without confirming', async () => {
		const onConfirm = vi.fn();
		const onClose = vi.fn();
		mount(RemoveApprovalDialog, { target: document.body, props: { open: true, name: 'pve-1', onConfirm, onClose } });
		await tick();

		buttonByText('Cancel')?.click();
		await tick();
		expect(onConfirm).not.toHaveBeenCalled();
		expect(onClose).toHaveBeenCalled();
	});
});
