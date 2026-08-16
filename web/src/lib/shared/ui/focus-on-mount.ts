import { tick } from 'svelte';

/**
 * focusOnMount — a Svelte action that moves focus to the attached element
 * on mount when enabled and no other element is already focused inside it.
 * Useful for focus management on prominent auth/error screens without
 * relying on the HTML autofocus attribute.
 */
export function focusOnMount(node: HTMLElement, enabled: boolean = true): void {
	if (!enabled) return;
	void tick().then(() => {
		if (document.activeElement !== node && !node.contains(document.activeElement)) {
			node.focus();
		}
	});
}
