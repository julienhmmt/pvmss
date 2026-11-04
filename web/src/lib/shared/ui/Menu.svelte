<script lang="ts">
	/**
	 * Menu - the shared overflow menu: `role="menu"` semantics, arrow-key
	 * navigation, Escape and outside-click dismissal, focus return to the
	 * trigger, and viewport-aware placement.
	 *
	 * Items carry an optional icon and a `destructive` flag for actions like
	 * Delete - the flag tints the label, it never skips the caller's own
	 * confirmation step.
	 */
	import { tick, type Component, type Snippet } from 'svelte';
	import MoreIcon from './icons/MoreIcon.svelte';

	export interface MenuItem {
		label: string;
		onclick: () => void;
		/** Tinted destructive entry (delete-style actions). */
		destructive?: boolean;
		disabled?: boolean;
		icon?: Component<{ class?: string }>;
		testId?: string;
	}

	interface Props {
		/** Accessible label on the ellipsis trigger. */
		label: string;
		items: readonly MenuItem[];
		testId?: string;
		/** Custom trigger content; defaults to the ellipsis icon. */
		trigger?: Snippet;
	}

	let { label, items, testId, trigger }: Props = $props();

	const TRIGGER_GAP = 4;
	const VIEWPORT_MARGIN = 8;

	let triggerEl: HTMLButtonElement | null = $state(null);
	let menuEl: HTMLElement | null = $state(null);
	let open = $state(false);
	let left = $state(0);
	let top = $state(0);

	function enabledItems(): HTMLElement[] {
		return [...(menuEl?.querySelectorAll<HTMLElement>('[role="menuitem"]:not([disabled])') ?? [])];
	}

	async function openMenu(focusLast = false): Promise<void> {
		open = true;
		await tick();
		if (!triggerEl || !menuEl) return;
		const rect = triggerEl.getBoundingClientRect();
		const { offsetWidth: width, offsetHeight: height } = menuEl;
		const below = rect.bottom + TRIGGER_GAP;
		const fitsBelow = below + height <= window.innerHeight - VIEWPORT_MARGIN;
		top = fitsBelow ? below : Math.max(VIEWPORT_MARGIN, rect.top - TRIGGER_GAP - height);
		left = Math.max(VIEWPORT_MARGIN, Math.min(rect.right - width, window.innerWidth - VIEWPORT_MARGIN - width));
		const enabled = enabledItems();
		(focusLast ? enabled.at(-1) : enabled[0])?.focus();
	}

	function close(returnFocus = true): void {
		if (!open) return;
		open = false;
		if (returnFocus) triggerEl?.focus();
	}

	function select(item: MenuItem): void {
		close();
		item.onclick();
	}

	function handleTriggerKeydown(event: KeyboardEvent): void {
		if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
		event.preventDefault();
		void openMenu(event.key === 'ArrowUp');
	}

	function handleMenuKeydown(event: KeyboardEvent): void {
		if (event.key === 'Escape') {
			event.preventDefault();
			event.stopPropagation();
			close();
			return;
		}
		if (event.key === 'Tab') {
			close(false);
			return;
		}
		const enabled = enabledItems();
		if (enabled.length === 0) return;
		const current = enabled.indexOf(document.activeElement as HTMLElement);
		const next: Record<string, number> = {
			ArrowDown: (current + 1) % enabled.length,
			ArrowUp: (current - 1 + enabled.length) % enabled.length,
			Home: 0,
			End: enabled.length - 1
		};
		const target = next[event.key];
		if (target === undefined) return;
		event.preventDefault();
		enabled[target]?.focus();
	}

	function handleOutsidePointer(event: PointerEvent): void {
		const target = event.target as Node;
		if (!open || menuEl?.contains(target) || triggerEl?.contains(target)) return;
		close(false);
	}
</script>

<svelte:window
	onpointerdown={handleOutsidePointer}
	onresize={() => close(false)}
	onscrollcapture={() => close(false)}
/>

<button
	bind:this={triggerEl}
	type="button"
	aria-label={label}
	aria-haspopup="menu"
	aria-expanded={open}
	data-testid={testId}
	onclick={() => (open ? close() : void openMenu())}
	onkeydown={handleTriggerKeydown}
	class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-[var(--radius-control)] border border-muted-foreground-subtle/50 bg-card text-foreground transition-colors pv-focus hover:border-muted-foreground-subtle hover:bg-muted"
>
	{#if trigger}
		{@render trigger()}
	{:else}
		<MoreIcon class="h-4 w-4" />
	{/if}
</button>

{#if open}
	<div
		bind:this={menuEl}
		role="menu"
		tabindex="-1"
		aria-label={label}
		onkeydown={handleMenuKeydown}
		class="fixed z-[9999] min-w-40 rounded-lg border border-border bg-card p-1 text-card-foreground shadow-overlay"
		style="left: {left}px; top: {top}px;"
	>
		{#each items as item (item.label)}
			<button
				type="button"
				role="menuitem"
				tabindex="-1"
				disabled={item.disabled ?? false}
				onclick={() => select(item)}
				data-testid={item.testId}
				class="flex w-full cursor-pointer items-center gap-2 rounded-[var(--radius-control)] px-2 py-1.5 text-left text-sm outline-none select-none hover:bg-muted focus:bg-muted disabled:pointer-events-none disabled:opacity-50 {item.destructive
					? 'text-destructive'
					: 'text-foreground'}"
			>
				{#if item.icon}
					<item.icon class="h-4 w-4" />
				{/if}
				{item.label}
			</button>
		{/each}
	</div>
{/if}
