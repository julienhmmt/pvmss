import { afterEach, beforeEach, describe, it, expect, vi } from 'vitest';
import { mount, tick } from 'svelte';
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
			...overrides
		},
		quota: { maxVmPerUser: -1 }
	};
}

function mountForm(onSave = vi.fn(), saving = false, policy = buildPolicy()) {
	mount(PolicyForm, {
		target: document.body,
		props: { policy, saving, saveError: null, onSave }
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
		const onSave = mountForm();
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

	it('manual save sends the full patch even when the form is clean', async () => {
		const onSave = mountForm();
		await tick();
		(document.querySelector('form') as HTMLFormElement).requestSubmit();
		await tick();
		expect(onSave).toHaveBeenCalledTimes(1);
		expect(onSave).toHaveBeenCalledWith({
			gabarit: { maxSockets: 4, maxCores: 8, maxMemoryMB: 16384, maxDiskPerVmGb: 500, maxNetworkCards: 4, maxSnapshots: 5, isolationVlanTag: 0 },
			quota: { maxVmPerUser: -1 }
		});
	});

	it('manual save cancels the pending debounce so only one save is sent', async () => {
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

	it('renders the manual save button and no discard control', () => {
		mountForm();
		expect(document.querySelector('button[type="submit"]')).not.toBeNull();
		const labels = Array.from(document.querySelectorAll('button')).map((b) => b.textContent ?? '');
		expect(labels.some((text) => text.includes('Annuler les modifications'))).toBe(false);
	});
});
