import { describe, it, expect, vi } from 'vitest';
import { mount, tick } from 'svelte';
import PolicyForm from './PolicyForm.svelte';
import type { AdminPolicy } from './policy.svelte';

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
			allowCustomYaml: false,
			...overrides
		},
		quota: { maxVmPerUser: -1 }
	};
}

function buildProps(policy: AdminPolicy, onSave = vi.fn()) {
	return {
		target: document.body,
		props: { policy, saving: false, saveError: null, onSave }
	};
}

function getNumberInputs(): HTMLInputElement[] {
	return Array.from(document.querySelectorAll('input[type="number"]'));
}

function getFirstInput(): HTMLInputElement {
	const inputs = getNumberInputs();
	if (inputs.length === 0) throw new Error('no number inputs found');
	return inputs[0] as HTMLInputElement;
}

function getQuotaInput(): HTMLInputElement {
	const inputs = getNumberInputs();
	if (inputs.length === 0) throw new Error('no number inputs found');
	return inputs[inputs.length - 1] as HTMLInputElement;
}

function getCheckbox(): HTMLInputElement {
	return document.querySelector('input[type="checkbox"]') as HTMLInputElement;
}

function getSubmitButton(): HTMLButtonElement {
	return Array.from(document.querySelectorAll('button[type="submit"]'))[0] as HTMLButtonElement;
}

function getDiscardButton(): HTMLButtonElement | undefined {
	return Array.from(document.querySelectorAll('button')).find((b) =>
		b.textContent?.includes('Annuler les modifications')
	) as HTMLButtonElement | undefined;
}

function setInputValue(input: HTMLInputElement, value: string): void {
	input.value = value;
	input.dispatchEvent(new Event('input', { bubbles: true }));
}

describe('PolicyForm', () => {
	it('disables the save button when the form is clean', () => {
		mount(PolicyForm, buildProps(buildPolicy()));
		expect(getSubmitButton().disabled).toBe(true);
		document.body.innerHTML = '';
	});

	it('enables the save button after a field is edited', async () => {
		mount(PolicyForm, buildProps(buildPolicy()));
		setInputValue(getFirstInput(), '6');
		await tick();
		expect(getSubmitButton().disabled).toBe(false);
		document.body.innerHTML = '';
	});

	it('disables the save button again after discard', async () => {
		mount(PolicyForm, buildProps(buildPolicy()));
		setInputValue(getFirstInput(), '6');
		await tick();
		expect(getSubmitButton().disabled).toBe(false);

		const discardBtn = getDiscardButton();
		expect(discardBtn).toBeDefined();
		discardBtn?.click();
		await tick();

		expect(getSubmitButton().disabled).toBe(true);
		document.body.innerHTML = '';
	});

	it('resets the field value to the original after discard', async () => {
		mount(PolicyForm, buildProps(buildPolicy()));
		setInputValue(getFirstInput(), '6');
		await tick();
		expect(getFirstInput().value).toBe('6');

		getDiscardButton()?.click();
		await tick();

		expect(getFirstInput().value).toBe('4');
		document.body.innerHTML = '';
	});

	it('detects dirty state when the checkbox is toggled', async () => {
		mount(PolicyForm, buildProps(buildPolicy()));
		expect(getSubmitButton().disabled).toBe(true);

		const checkbox = getCheckbox();
		checkbox.checked = true;
		checkbox.dispatchEvent(new Event('change', { bubbles: true }));
		await tick();

		expect(getSubmitButton().disabled).toBe(false);
		document.body.innerHTML = '';
	});

	it('detects dirty state when the quota field is edited', async () => {
		mount(PolicyForm, buildProps(buildPolicy()));
		setInputValue(getQuotaInput(), '5');
		await tick();
		expect(getSubmitButton().disabled).toBe(false);
		document.body.innerHTML = '';
	});

	it('calls onSave with the full gabarit and quota on submit', async () => {
		const onSave = vi.fn();
		mount(PolicyForm, buildProps(buildPolicy(), onSave));

		setInputValue(getFirstInput(), '6');
		await tick();

		const form = document.querySelector('form') as HTMLFormElement;
		form.requestSubmit();
		await tick();

		expect(onSave).toHaveBeenCalledTimes(1);
		expect(onSave).toHaveBeenCalledWith({
			gabarit: expect.objectContaining({ maxSockets: 6 }),
			quota: { maxVmPerUser: -1 }
		});
		document.body.innerHTML = '';
	});

	it('does not show the discard button when the form is clean', () => {
		mount(PolicyForm, buildProps(buildPolicy()));
		expect(getDiscardButton()).toBeUndefined();
		document.body.innerHTML = '';
	});

	it('disables the save button while saving', () => {
		mount(PolicyForm, {
			target: document.body,
			props: { policy: buildPolicy(), saving: true, saveError: null, onSave: vi.fn() }
		});
		expect(getSubmitButton().disabled).toBe(true);
		document.body.innerHTML = '';
	});
});
