import { describe, it, expect, afterEach, vi } from 'vitest';
import { mount } from 'svelte';
import ModeChooser from './ModeChooser.svelte';
import { m } from '$lib/paraglide/messages.js';

function cards(): HTMLButtonElement[] {
	return [...document.body.querySelectorAll('button')];
}

describe('ModeChooser', () => {
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('renders one card per mode', () => {
		mount(ModeChooser, { target: document.body, props: { onSelect: () => {} } });

		expect(cards()).toHaveLength(2);
		expect(cards().map((card) => card.dataset.testid)).toEqual([
			'create-mode-simple',
			'create-mode-detailed'
		]);
	});

	it('labels the group', () => {
		mount(ModeChooser, { target: document.body, props: { onSelect: () => {} } });

		const group = document.body.querySelector('[role="group"]');
		expect(group).not.toBeNull();
		const labelId = group?.getAttribute('aria-labelledby') ?? '';
		expect(document.getElementById(labelId)?.textContent?.trim()).toBe(m['vms.create.modeLabel']());
	});

	it('fires onSelect with the chosen mode', () => {
		const onSelect = vi.fn();
		mount(ModeChooser, { target: document.body, props: { onSelect } });

		cards()[0]?.click();

		expect(onSelect).toHaveBeenCalledTimes(1);
		expect(onSelect).toHaveBeenCalledWith('simple');
	});
});
