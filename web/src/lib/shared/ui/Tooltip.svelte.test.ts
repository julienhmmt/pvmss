import { describe, it, expect } from 'vitest';
import { mount } from 'svelte';
import Tooltip from './Tooltip.svelte';

describe('Tooltip', () => {
	it('renders the tooltip text with role="tooltip"', () => {
		mount(Tooltip, {
			target: document.body,
			props: { text: 'Helpful hint' }
		});

		const tooltip = document.body.querySelector('[role="tooltip"]');
		expect(tooltip).not.toBeNull();
		expect(tooltip!.textContent).toContain('Helpful hint');
	});

	it('links the trigger to the tooltip via aria-describedby', () => {
		mount(Tooltip, {
			target: document.body,
			props: { text: 'Linked hint' }
		});

		const wrapper = document.body.querySelector('[aria-describedby]');
		expect(wrapper).not.toBeNull();
		const describedById = wrapper!.getAttribute('aria-describedby');
		expect(describedById).toBeTruthy();
		const tooltip = document.body.querySelector(`#${describedById}`);
		expect(tooltip).not.toBeNull();
	});
});
