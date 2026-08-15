/**
 * trapFocus — a Svelte action that confines Tab navigation to the element's
 * focusable descendants while it is mounted, and moves focus into the element
 * on mount. Used by the sidebar drawer (T035). Call `node.focus()` on the
 * container (it should have tabindex="-1").
 */
const FOCUSABLE =
	'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

export function trapFocus(node: HTMLElement): void | { destroy(): void } {
	const previouslyFocused = document.activeElement;
	node.focus();
	function onKeydown(event: KeyboardEvent): void {
		if (event.key !== 'Tab') return;
		const focusable = Array.from(node.querySelectorAll<HTMLElement>(FOCUSABLE));
		if (focusable.length === 0) {
			event.preventDefault();
			return;
		}
		const first = focusable[0];
		const last = focusable[focusable.length - 1];
		if (first === undefined || last === undefined) return;
		const active = document.activeElement;
		if (event.shiftKey) {
			if (active === first || !node.contains(active)) {
				event.preventDefault();
				last.focus();
			}
		} else if (active === last) {
			event.preventDefault();
			first.focus();
		}
	}
	node.addEventListener('keydown', onKeydown);
	// Restore focus to the trigger when the drawer unmounts.
	return {
		destroy(): void {
			node.removeEventListener('keydown', onKeydown);
			if (previouslyFocused instanceof HTMLElement) {
				previouslyFocused.focus();
			}
		}
	};
}
