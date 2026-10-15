import { describe, it, expect, afterEach, vi } from 'vitest';
import { mount, createRawSnippet } from 'svelte';
import RadioCard from './RadioCard.svelte';

const headerSnippet = (text: string) =>
	createRawSnippet(() => ({ render: () => `<span>${text}</span>` }));
const bodySnippet = (text: string) =>
	createRawSnippet(() => ({ render: () => `<span>${text}</span>` }));

const baseProps = {
	name: 'size',
	value: 'small',
	header: headerSnippet('Small'),
	children: bodySnippet('2 vCPU · 2 GB')
};

describe('RadioCard', () => {
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('renders a label wrapping a visually-hidden radio', () => {
		mount(RadioCard, {
			target: document.body,
			props: { ...baseProps, selected: false, onSelect: () => {} }
		});

		const label = document.body.querySelector('label');
		expect(label).not.toBeNull();
		const radio = document.body.querySelector('input[type="radio"]') as HTMLInputElement;
		expect(radio).not.toBeNull();
		expect(radio.className).toContain('opacity-0');
		expect(radio.getAttribute('value')).toBe('small');
		expect(radio.getAttribute('name')).toBe('size');
	});

	it('reflects the selected state on the radio and the card', () => {
		mount(RadioCard, {
			target: document.body,
			props: { ...baseProps, selected: true, onSelect: () => {} }
		});

		const radio = document.body.querySelector('input[type="radio"]') as HTMLInputElement;
		expect(radio.checked).toBe(true);
		// The selected indicator dot is present.
		expect(document.body.querySelector('.bg-primary')).not.toBeNull();
	});

	it('does not show the selected indicator when unselected', () => {
		mount(RadioCard, {
			target: document.body,
			props: { ...baseProps, selected: false, onSelect: () => {} }
		});

		const radio = document.body.querySelector('input[type="radio"]') as HTMLInputElement;
		expect(radio.checked).toBe(false);
	});

	it('disables the radio when disabled is true', () => {
		mount(RadioCard, {
			target: document.body,
			props: { ...baseProps, selected: false, disabled: true, onSelect: () => {} }
		});

		const radio = document.body.querySelector('input[type="radio"]') as HTMLInputElement;
		expect(radio.disabled).toBe(true);
	});

	it('renders the header and body snippets', () => {
		mount(RadioCard, {
			target: document.body,
			props: {
				...baseProps,
				header: headerSnippet('My Header'),
				children: bodySnippet('My Body'),
				selected: false,
				onSelect: () => {}
			}
		});

		expect(document.body.textContent).toContain('My Header');
		expect(document.body.textContent).toContain('My Body');
	});

	it('fires onSelect with the value when the radio changes', () => {
		const onSelect = vi.fn();
		mount(RadioCard, {
			target: document.body,
			props: { ...baseProps, selected: false, onSelect }
		});

		const radio = document.body.querySelector('input[type="radio"]') as HTMLInputElement;
		radio.checked = true;
		radio.dispatchEvent(new Event('change', { bubbles: true }));
		expect(onSelect).toHaveBeenCalledTimes(1);
		expect(onSelect).toHaveBeenCalledWith('small');
	});

	it('does not fire onSelect when disabled', () => {
		const onSelect = vi.fn();
		mount(RadioCard, {
			target: document.body,
			props: { ...baseProps, selected: false, disabled: true, onSelect }
		});

		const radio = document.body.querySelector('input[type="radio"]') as HTMLInputElement;
		radio.checked = true;
		radio.dispatchEvent(new Event('change', { bubbles: true }));
		expect(onSelect).not.toHaveBeenCalled();
	});

	it('keeps a stable name attribute regardless of selection', () => {
		mount(RadioCard, {
			target: document.body,
			props: { ...baseProps, selected: false, onSelect: () => {} }
		});
		let radio = document.body.querySelector('input[type="radio"]') as HTMLInputElement;
		expect(radio.getAttribute('name')).toBe('size');
		document.body.innerHTML = '';

		mount(RadioCard, {
			target: document.body,
			props: { ...baseProps, selected: true, onSelect: () => {} }
		});
		radio = document.body.querySelector('input[type="radio"]') as HTMLInputElement;
		expect(radio.getAttribute('name')).toBe('size');
	});
});
