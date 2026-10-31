import { afterEach, beforeEach, describe, it, expect, vi } from 'vitest';
import { mount, tick, unmount } from 'svelte';
import PolicyForm from './PolicyForm.svelte';
import type { AdminPolicy, AdminPolicyPatch } from './policy.svelte';

const DEBOUNCE_MS = 700;

function buildPolicy(overrides: Partial<AdminPolicy['gabarit']> = {}): AdminPolicy {
	return {
		cluster: 'default',
		gabarit: {
			maxSockets: 4,
			maxCores: 8,
			maxMemoryMB: 16384,
			maxDiskPerVmGb: 500,
			maxNetworkCards: 4,
			maxSnapshots: 5,
			isolationVlanTag: 0,
			vmidMin: 0,
			vmidMax: 0,
			...overrides
		},
		quota: { maxVmPerUser: -1 }
	};
}

function mountForm(onSave = vi.fn(), saving = false, policy = buildPolicy(), saveError: string | null = null) {
	mount(PolicyForm, {
		target: document.body,
		props: { policy, saving, saveError, onSave }
	});
	return onSave;
}

function getNumberInputs(): HTMLInputElement[] {
	return Array.from(document.querySelectorAll('input[type="number"]'));
}

function getFirstInput(): HTMLInputElement {
	const input = getNumberInputs()[0];
	if (!input) throw new Error('no number inputs found');
	return input;
}

function getQuotaInput(): HTMLInputElement {
	const inputs = getNumberInputs();
	const input = inputs[inputs.length - 1];
	if (!input) throw new Error('no number inputs found');
	return input;
}

function setInputValue(input: HTMLInputElement, value: string): void {
	input.value = value;
	input.dispatchEvent(new Event('input', { bubbles: true }));
}

describe('PolicyForm autosave', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
		document.body.innerHTML = '';
	});

	it('does not save when the form is clean', async () => {
		const onSave = mountForm();
		await tick();
		vi.advanceTimersByTime(DEBOUNCE_MS * 3);
		expect(onSave).not.toHaveBeenCalled();
	});

	it('saves only after the debounce window has elapsed', async () => {
		const onSave = mountForm();
		setInputValue(getFirstInput(), '6');
		await tick();

		vi.advanceTimersByTime(DEBOUNCE_MS - 1);
		expect(onSave).not.toHaveBeenCalled();

		vi.advanceTimersByTime(1);
		expect(onSave).toHaveBeenCalledTimes(1);
		expect(onSave).toHaveBeenCalledWith({
			gabarit: expect.objectContaining({ maxSockets: 6 }) as { maxSockets: number },
			quota: { maxVmPerUser: -1 }
		});
	});

	it('sends one save for a burst of edits, with the last value', async () => {
		const onSave = mountForm();
		const input = getFirstInput();
		setInputValue(input, '5');
		await tick();
		vi.advanceTimersByTime(200);
		setInputValue(input, '6');
		await tick();
		vi.advanceTimersByTime(200);
		setInputValue(input, '7');
		await tick();

		vi.advanceTimersByTime(DEBOUNCE_MS);
		expect(onSave).toHaveBeenCalledTimes(1);
		const patch = onSave.mock.calls[0]?.[0] as AdminPolicyPatch | undefined;
		expect(patch?.gabarit?.maxSockets).toBe(7);
	});

	it('does not save when a field is cleared to an invalid value', async () => {
		const onSave = mountForm();
		setInputValue(getFirstInput(), '');
		await tick();
		vi.advanceTimersByTime(DEBOUNCE_MS * 3);
		expect(onSave).not.toHaveBeenCalled();
	});

	it('autosaves the quota field through the same path', async () => {
		const onSave = mountForm(vi.fn(), false, { ...buildPolicy(), quota: { maxVmPerUser: 3 } });
		setInputValue(getQuotaInput(), '5');
		await tick();
		vi.advanceTimersByTime(DEBOUNCE_MS);
		expect(onSave).toHaveBeenCalledTimes(1);
		expect(onSave).toHaveBeenCalledWith({
			gabarit: expect.objectContaining({ maxSockets: 4 }) as { maxSockets: number },
			quota: { maxVmPerUser: 5 }
		});
	});

	it('does not start a save while one is in flight', async () => {
		const onSave = mountForm(vi.fn(), true);
		setInputValue(getFirstInput(), '6');
		await tick();
		vi.advanceTimersByTime(DEBOUNCE_MS * 3);
		expect(onSave).not.toHaveBeenCalled();
	});

	it('does not resend the same patch after it was sent', async () => {
		const onSave = mountForm();
		setInputValue(getFirstInput(), '6');
		await tick();
		vi.advanceTimersByTime(DEBOUNCE_MS);
		expect(onSave).toHaveBeenCalledTimes(1);

		vi.advanceTimersByTime(DEBOUNCE_MS * 3);
		expect(onSave).toHaveBeenCalledTimes(1);
	});

	it('submitting (Enter) sends the full patch even when the form is clean', async () => {
		const onSave = mountForm();
		await tick();
		(document.querySelector('form') as HTMLFormElement).requestSubmit();
		await tick();
		expect(onSave).toHaveBeenCalledTimes(1);
		expect(onSave).toHaveBeenCalledWith({
			gabarit: { maxSockets: 4, maxCores: 8, maxMemoryMB: 16384, maxDiskPerVmGb: 500, maxNetworkCards: 4, maxSnapshots: 5, isolationVlanTag: 0, vmidMin: 0, vmidMax: 0 },
			quota: { maxVmPerUser: -1 }
		});
	});

	it('submitting cancels the pending debounce so only one save is sent', async () => {
		const onSave = mountForm();
		setInputValue(getFirstInput(), '6');
		await tick();
		(document.querySelector('form') as HTMLFormElement).requestSubmit();
		await tick();
		vi.advanceTimersByTime(DEBOUNCE_MS * 3);
		expect(onSave).toHaveBeenCalledTimes(1);
		const patch = onSave.mock.calls[0]?.[0] as AdminPolicyPatch | undefined;
		expect(patch?.gabarit?.maxSockets).toBe(6);
	});

	it('has no save button: autosave is the only save model', () => {
		mountForm();
		expect(document.querySelector('button[type="submit"]')).toBeNull();
	});
});

describe('PolicyForm unlimited quota and GiB memory', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
		document.body.innerHTML = '';
	});

	const quotaSwitch = (): HTMLButtonElement => {
		const sw = document.querySelector<HTMLButtonElement>('button[role="switch"]');
		if (!sw) throw new Error('no switch found');
		return sw;
	};

	it('switching Unlimited on submits -1 and disables the number input', async () => {
		const onSave = mountForm(vi.fn(), false, { ...buildPolicy(), quota: { maxVmPerUser: 5 } });
		await tick();

		expect(getQuotaInput().disabled).toBe(false);
		quotaSwitch().click();
		await tick();

		expect(getQuotaInput().disabled).toBe(true);
		vi.advanceTimersByTime(DEBOUNCE_MS);
		expect(onSave).toHaveBeenCalledWith(expect.objectContaining({ quota: { maxVmPerUser: -1 } }));
	});

	it('switching Unlimited off restores the last positive value', async () => {
		const onSave = mountForm(vi.fn(), false, { ...buildPolicy(), quota: { maxVmPerUser: 5 } });
		await tick();

		quotaSwitch().click();
		await tick();
		quotaSwitch().click();
		await tick();

		// Back to the original value - clean form, enabled input showing 5.
		expect(getQuotaInput().disabled).toBe(false);
		expect(getQuotaInput().value).toBe('5');
		vi.advanceTimersByTime(DEBOUNCE_MS);
		expect(onSave).not.toHaveBeenCalled();
	});

	it('displays memory in GiB and submits the value in MB', async () => {
		const onSave = mountForm(vi.fn(), false, buildPolicy({ maxMemoryMB: 16384 }));
		await tick();

		const memoryInput = getNumberInputs()[2];
		if (!memoryInput) throw new Error('no memory input found');
		expect(memoryInput.value).toBe('16');

		setInputValue(memoryInput, '2');
		await tick();
		vi.advanceTimersByTime(DEBOUNCE_MS);
		expect(onSave.mock.calls.at(-1)?.[0]).toMatchObject({ gabarit: { maxMemoryMB: 2048 } });
	});
});

describe('PolicyForm save status', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
		document.body.innerHTML = '';
	});

	const status = (): string | undefined => document.querySelector<HTMLElement>('[data-testid="policy-save-status"]')?.dataset.status;
	const retry = (): HTMLButtonElement | null => document.querySelector('[data-testid="policy-save-retry"]');

	it('reads saved on a clean form', async () => {
		mountForm();
		await tick();
		expect(status()).toBe('saved');
		expect(retry()).toBeNull();
	});

	it('reads unsaved while the debounce is pending', async () => {
		mountForm();
		setInputValue(getFirstInput(), '6');
		await tick();
		expect(status()).toBe('unsaved');
	});

	it('reads invalid when a field blocks the save', async () => {
		mountForm();
		setInputValue(getFirstInput(), '');
		await tick();
		expect(status()).toBe('invalid');
	});

	it('reads saving while a save is in flight', async () => {
		mountForm(vi.fn(), true);
		await tick();
		expect(status()).toBe('saving');
	});

	it('reads failed with a retry that resends the full patch', async () => {
		const onSave = mountForm(vi.fn(), false, buildPolicy(), 'boom');
		await tick();
		expect(status()).toBe('failed');

		retry()?.click();
		expect(onSave).toHaveBeenCalledTimes(1);
		expect(onSave.mock.calls[0]?.[0]).toMatchObject({ gabarit: { maxSockets: 4 }, quota: { maxVmPerUser: -1 } });
	});
});

describe('PolicyForm unmount', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
		document.body.innerHTML = '';
	});

	it('sends a pending edit when the form unmounts before the debounce', async () => {
		const onSave = vi.fn();
		const form = mount(PolicyForm, {
			target: document.body,
			props: { policy: buildPolicy(), saving: false, saveError: null, onSave }
		});
		setInputValue(getFirstInput(), '6');
		await tick();

		vi.advanceTimersByTime(DEBOUNCE_MS - 100);
		await unmount(form);

		expect(onSave).toHaveBeenCalledTimes(1);
		expect(onSave.mock.calls[0]?.[0]).toMatchObject({ gabarit: { maxSockets: 6 } });
	});
});
