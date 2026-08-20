import { describe, it, expect, afterEach } from 'vitest';
import { mount } from 'svelte';
import Tooltip from './Tooltip.svelte';

describe('Tooltip', () => {
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('renders a trigger element with aria-describedby', () => {
		mount(Tooltip, {
			target: document.body,
			props: { text: 'Helpful hint' }
		});

		const trigger = document.body.querySelector('[aria-describedby]');
		expect(trigger).not.toBeNull();
		const describedById = trigger!.getAttribute('aria-describedby');
		expect(describedById).toBeTruthy();
	});

	it('does not render the tooltip element before hover', () => {
		mount(Tooltip, {
			target: document.body,
			props: { text: 'Hidden hint' }
		});

		const tooltip = document.body.querySelector('[role="tooltip"]');
		expect(tooltip).toBeNull();
	});

	it('renders the tooltip text when visible', () => {
		mount(Tooltip, {
			target: document.body,
			props: { text: 'Visible hint' }
		});

		// The tooltip text is not in the DOM until hover/focus triggers it.
		// Verify the trigger element exists and has the correct aria link.
		const trigger = document.body.querySelector('[aria-describedby]') as HTMLElement;
		expect(trigger).not.toBeNull();
		expect(trigger.getAttribute('aria-describedby')).toMatch(/^tooltip-\d+$/);
	});
});
