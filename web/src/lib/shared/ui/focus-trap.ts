import { tick } from 'svelte';
import type { Action, ActionReturn } from 'svelte/action';

/**
 * CSS selector for the elements that can receive keyboard focus inside a
 * container. Matches the focusable set used by assistive technology without
 * relying on a third-party library.
 */
const FOCUSABLE_SELECTORS =
	'button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])';

/**
 * Returns true when an element is visible enough to be focused.
 */
function isVisible(element: HTMLElement): boolean {
	if (element.hidden) return false;
	if (element.getAttribute('aria-hidden') === 'true') return false;
	const style = globalThis.getComputedStyle(element);
	return style.display !== 'none' && style.visibility !== 'hidden';
}

/**
 * Collects all currently visible and focusable descendants of a node,
 * ordered by DOM position.
 */
function getFocusableElements(node: HTMLElement): HTMLElement[] {
	const elements = Array.from(node.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTORS));
	if (typeof globalThis === 'undefined') return elements;
	return elements.filter(isVisible);
}

/**
 * Focus-trap Svelte action.
 *
 * On mount it focuses the first focusable descendant of the attached element,
 * falling back to the element itself when no focusable child exists. Tab and
 * Shift+Tab wrap focus within the element, and the originally focused element
 * is restored when the action is destroyed (i.e. when the dialog closes).
 */
export const focusTrap = ((node: HTMLElement): ActionReturn => {
	const trigger = document.activeElement;

	function handleKeydown(event: KeyboardEvent) {
		if (event.key !== 'Tab' || event.altKey || event.ctrlKey || event.metaKey) return;

		const focusable = getFocusableElements(node);
		if (focusable.length === 0) {
			event.preventDefault();
			node.focus();
			return;
		}

		const current = document.activeElement as HTMLElement | null;
		const currentIndex = current ? focusable.indexOf(current) : -1;

		const nextIndex = event.shiftKey
			? prevTabIndex(currentIndex, focusable.length)
			: nextTabIndex(currentIndex, focusable.length);

		event.preventDefault();
		focusable[nextIndex]!.focus();
	}

	node.addEventListener('keydown', handleKeydown);

	void tick().then(() => {
		const focusable = getFocusableElements(node);
		if (focusable.length > 0) {
			focusable[0]!.focus();
		} else if (!node.contains(document.activeElement)) {
			node.focus();
		}
	});

	return {
		destroy() {
			node.removeEventListener('keydown', handleKeydown);
			if (trigger instanceof HTMLElement && document.body.contains(trigger)) {
				trigger.focus();
			}
		}
	};
}) satisfies Action<HTMLElement>;

/** Computes the tab index to focus when Shift+Tab wraps past the start. */
function prevTabIndex(currentIndex: number, length: number): number {
	if (currentIndex <= 0) return length - 1;
	return currentIndex - 1;
}

/** Computes the tab index to focus when Tab wraps past the end. */
function nextTabIndex(currentIndex: number, length: number): number {
	if (currentIndex === -1 || currentIndex === length - 1) return 0;
	return currentIndex + 1;
}
