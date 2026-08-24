import { describe, it, expect, afterEach, vi } from 'vitest';
import { mount, unmount, tick, createRawSnippet } from 'svelte';
import Dialog from './Dialog.svelte';

const children = createRawSnippet(() => ({
	render: () =>
		'<div><h2 id="dialog-title">Test dialog</h2><button data-testid="first-focusable" type="button">First</button><button data-testid="second-focusable" type="button">Second</button></div>'
}));

function dispatchKey(element: Element, key: string, shiftKey = false): void {
	element.dispatchEvent(
		new KeyboardEvent('keydown', { key, shiftKey, bubbles: true, cancelable: true })
	);
}

describe('Dialog', () => {
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('focuses the first focusable element, traps Tab, and restores focus on close', async () => {
		document.body.innerHTML = '<button id="trigger" type="button">Open</button><div id="host"></div>';

		const trigger = document.querySelector<HTMLButtonElement>('#trigger');
		expect(trigger).not.toBeNull();

		trigger!.focus();

		const dialog = mount(Dialog, {
			target: document.querySelector<HTMLElement>('#host')!,
			props: {
				open: true,
				labelledBy: 'dialog-title',
				onClose: vi.fn(),
				children
			}
		});
		await tick();

		const first = document.querySelector<HTMLButtonElement>('[data-testid="first-focusable"]');
		const second = document.querySelector<HTMLButtonElement>('[data-testid="second-focusable"]');
		expect(first).not.toBeNull();
		expect(second).not.toBeNull();

		expect(document.activeElement).toBe(first);

		// Tab moves to the second focusable element.
		dispatchKey(first!, 'Tab');
		expect(document.activeElement).toBe(second);

		// Tab wraps from the last element back to the first.
		dispatchKey(second!, 'Tab');
		expect(document.activeElement).toBe(first);

		// Shift+Tab wraps from the first element to the last.
		dispatchKey(first!, 'Tab', true);
		expect(document.activeElement).toBe(second);

		// Closing the dialog returns focus to the trigger.
		await unmount(dialog);
		expect(document.activeElement).toBe(trigger);
	});
});
