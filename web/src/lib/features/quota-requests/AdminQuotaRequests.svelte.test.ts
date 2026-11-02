import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { mount, tick } from 'svelte';
import AdminQuotaRequests from './AdminQuotaRequests.svelte';
import {
	approveQuotaRequest,
	denyQuotaRequest,
	fetchAdminQuotaRequests,
	type QuotaRequest
} from './quota-requests';
import { setLocale } from '#lib/paraglide/runtime.js';

vi.mock('./quota-requests', () => ({
	approveQuotaRequest: vi.fn(),
	denyQuotaRequest: vi.fn(),
	fetchAdminQuotaRequests: vi.fn()
}));

const request = {
	id: 7,
	requester: 'alice',
	pool: 'lab',
	cluster: 'pve',
	currentQuota: 2,
	requestedQuota: 5,
	reason: 'more machines',
	status: 'pending'
} as unknown as QuotaRequest;

async function render(): Promise<void> {
	vi.mocked(fetchAdminQuotaRequests).mockResolvedValue([request]);
	mount(AdminQuotaRequests, { target: document.body });
	await vi.waitFor(() => expect(document.querySelector('[data-testid="quota-request-row"]')).not.toBeNull());
}

const rowButton = (label: string): HTMLButtonElement | undefined =>
	[...document.querySelectorAll<HTMLButtonElement>('[data-testid="quota-request-row"] button')].find(
		(b) => b.textContent?.trim() === label
	);

const confirmButton = (): HTMLButtonElement | null =>
	document.querySelector<HTMLButtonElement>('[data-testid="quota-decision-confirm"]');

describe('AdminQuotaRequests decisions', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
		vi.clearAllMocks();
	});

	it('asks before denying and denies only on confirm', async () => {
		await render();

		rowButton('Deny')?.click();
		await tick();
		expect(denyQuotaRequest).not.toHaveBeenCalled();
		expect(confirmButton()).not.toBeNull();

		confirmButton()?.click();
		await vi.waitFor(() => expect(denyQuotaRequest).toHaveBeenCalledTimes(1));
		expect(denyQuotaRequest).toHaveBeenCalledWith(7);
	});

	it('approves the requested number without a dialog when nothing was edited', async () => {
		vi.mocked(approveQuotaRequest).mockResolvedValue(undefined as never);
		await render();

		rowButton('Approve')?.click();
		await tick();

		expect(confirmButton()).toBeNull();
		expect(approveQuotaRequest).toHaveBeenCalledWith(7, undefined);
	});

	it('asks before approving a changed allowance and approves the typed number on confirm', async () => {
		vi.mocked(approveQuotaRequest).mockResolvedValue(undefined as never);
		await render();

		const input = document.querySelector<HTMLInputElement>('input[type="number"]');
		if (!input) throw new Error('allowance input missing');
		input.value = '8';
		input.dispatchEvent(new Event('input'));
		await tick();

		rowButton('Approve')?.click();
		await tick();
		expect(approveQuotaRequest).not.toHaveBeenCalled();
		expect(confirmButton()).not.toBeNull();

		confirmButton()?.click();
		await vi.waitFor(() => expect(approveQuotaRequest).toHaveBeenCalledWith(7, 8));
	});
});
