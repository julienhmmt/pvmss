import { afterEach, describe, expect, it, vi } from 'vitest';
import { focusTrap } from './focus-trap';

afterEach(() => vi.unstubAllGlobals());

function setupContainer(focusableCount: number): HTMLElement {
	const container = document.createElement('div');
	for (let i = 0; i < focusableCount; i++) {
		const btn = document.createElement('button');
		btn.textContent = `Button ${i + 1}`;
		container.appendChild(btn);
	}
	document.body.appendChild(container);
	return container;
}

describe('focusTrap', () => {
	it('focuses the first focusable element on mount', async () => {
		const container = setupContainer(2);
		vi.stubGlobal('getComputedStyle', () => ({ display: 'block', visibility: 'visible' }) as CSSStyleDeclaration);
		const cleanup = focusTrap(container);
		await new Promise((resolve) => setTimeout(resolve, 0));
		expect(document.activeElement).toBe(container.querySelector('button'));
		cleanup.destroy?.();
		container.remove();
	});

	it('focuses the container itself when no focusable child exists', async () => {
		const container = setupContainer(0);
		container.setAttribute('tabindex', '0');
		vi.stubGlobal('getComputedStyle', () => ({ display: 'block', visibility: 'visible' }) as CSSStyleDeclaration);
		const cleanup = focusTrap(container);
		await new Promise((resolve) => setTimeout(resolve, 0));
		expect(document.activeElement).toBe(container);
		cleanup.destroy?.();
		container.remove();
	});

	it('wraps focus forward on Tab from the last element', async () => {
		const container = setupContainer(2);
		vi.stubGlobal('getComputedStyle', () => ({ display: 'block', visibility: 'visible' }) as CSSStyleDeclaration);
		const cleanup = focusTrap(container);
		await new Promise((resolve) => setTimeout(resolve, 0));
		const buttons = container.querySelectorAll('button');
		buttons[1]!.focus();
		const event = new KeyboardEvent('keydown', { key: 'Tab' });
		const preventDefault = vi.spyOn(event, 'preventDefault');
		container.dispatchEvent(event);
		expect(preventDefault).toHaveBeenCalled();
		expect(document.activeElement).toBe(buttons[0]);
		cleanup.destroy?.();
		container.remove();
	});

	it('wraps focus backward on Shift+Tab from the first element', async () => {
		const container = setupContainer(2);
		vi.stubGlobal('getComputedStyle', () => ({ display: 'block', visibility: 'visible' }) as CSSStyleDeclaration);
		const cleanup = focusTrap(container);
		await new Promise((resolve) => setTimeout(resolve, 0));
		const buttons = container.querySelectorAll('button');
		buttons[0]!.focus();
		const event = new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true });
		const preventDefault = vi.spyOn(event, 'preventDefault');
		container.dispatchEvent(event);
		expect(preventDefault).toHaveBeenCalled();
		expect(document.activeElement).toBe(buttons[1]);
		cleanup.destroy?.();
		container.remove();
	});

	it('ignores non-Tab keys', async () => {
		const container = setupContainer(2);
		vi.stubGlobal('getComputedStyle', () => ({ display: 'block', visibility: 'visible' }) as CSSStyleDeclaration);
		const cleanup = focusTrap(container);
		await new Promise((resolve) => setTimeout(resolve, 0));
		const buttons = container.querySelectorAll('button');
		buttons[0]!.focus();
		const event = new KeyboardEvent('keydown', { key: 'Escape' });
		const preventDefault = vi.spyOn(event, 'preventDefault');
		container.dispatchEvent(event);
		expect(preventDefault).not.toHaveBeenCalled();
		cleanup.destroy?.();
		container.remove();
	});

	it('ignores Tab with modifier keys', async () => {
		const container = setupContainer(2);
		vi.stubGlobal('getComputedStyle', () => ({ display: 'block', visibility: 'visible' }) as CSSStyleDeclaration);
		const cleanup = focusTrap(container);
		await new Promise((resolve) => setTimeout(resolve, 0));
		const buttons = container.querySelectorAll('button');
		buttons[0]!.focus();
		const event = new KeyboardEvent('keydown', { key: 'Tab', ctrlKey: true });
		const preventDefault = vi.spyOn(event, 'preventDefault');
		container.dispatchEvent(event);
		expect(preventDefault).not.toHaveBeenCalled();
		cleanup.destroy?.();
		container.remove();
	});

	it('restores focus to the trigger on destroy', async () => {
		const trigger = document.createElement('button');
		trigger.textContent = 'Trigger';
		document.body.appendChild(trigger);
		trigger.focus();

		const container = setupContainer(1);
		vi.stubGlobal('getComputedStyle', () => ({ display: 'block', visibility: 'visible' }) as CSSStyleDeclaration);
		const cleanup = focusTrap(container);
		await new Promise((resolve) => setTimeout(resolve, 0));
		cleanup.destroy?.();
		expect(document.activeElement).toBe(trigger);
		container.remove();
		trigger.remove();
	});
});
