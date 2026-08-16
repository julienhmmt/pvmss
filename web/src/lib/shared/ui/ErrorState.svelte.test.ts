import { describe, it, expect, vi } from 'vitest';
import { mount } from 'svelte';
import ErrorState from './ErrorState.svelte';

describe('ErrorState', () => {
	it('renders the title and default error icon', () => {
		mount(ErrorState, {
			target: document.body,
			props: { title: 'Something failed' }
		});

		expect(document.body.textContent).toContain('Something failed');
		expect(document.body.querySelector('[role="alert"]')).not.toBeNull();
	});

	it('renders the description when provided', () => {
		mount(ErrorState, {
			target: document.body,
			props: { title: 'Failed', description: 'Try again later' }
		});

		expect(document.body.textContent).toContain('Try again later');
	});

	it('calls the retry callback when the retry button is clicked', () => {
		const retry = vi.fn();
		mount(ErrorState, {
			target: document.body,
			props: { title: 'Failed', retry }
		});

		const button = document.body.querySelector('button');
		expect(button).not.toBeNull();
		button!.click();
		expect(retry).toHaveBeenCalledOnce();
	});
});
