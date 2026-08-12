<script lang="ts">
	import type { Snippet } from 'svelte';

	/**
	 * Dropdown — a small accessible menu primitive in the same hand-rolled
	 * style as Dialog/Tabs/ClusterSelector (no bits-ui dependency in this
	 * codebase). Opens on click, closes on outside click or Escape, restores
	 * focus to the trigger, and renders its options via a snippet.
	 */
	interface Props {
		/** Accessible label for the trigger button. */
		label: string;
		/** Snippet rendering the menu options; receives a `close` callback. */
		options: Snippet<[() => void]>;
		/** Snippet rendering the trigger's inner content (icon/text). */
		trigger?: Snippet;
	}

	let { label, options, trigger }: Props = $props();

	let open = $state(false);
	let triggerEl = $state<HTMLButtonElement | null>(null);
	let menuEl = $state<HTMLDivElement | null>(null);

	function toggle(): void {
		open = !open;
		if (open) triggerEl?.focus();
	}

	function close(): void {
		open = false;
		triggerEl?.focus();
	}

	function handleTriggerKeydown(event: KeyboardEvent): void {
		if (event.key === 'Escape' && open) {
			event.preventDefault();
			close();
		}
	}

	function handleMenuKeydown(event: KeyboardEvent): void {
		if (event.key === 'Escape') {
			event.preventDefault();
			close();
		}
	}

	function handleWindowClick(event: MouseEvent): void {
		if (!open) return;
		const target = event.target as Node | null;
		if (menuEl && target && !menuEl.contains(target) && triggerEl && !triggerEl.contains(target)) {
			close();
		}
	}
</script>

<svelte:window onclick={handleWindowClick} />

<div class="relative" bind:this={menuEl}>
	<button
		type="button"
		bind:this={triggerEl}
		onclick={toggle}
		onkeydown={handleTriggerKeydown}
		aria-haspopup="menu"
		aria-expanded={open}
		aria-label={label}
		class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-sm text-muted-foreground hover:text-foreground"
	>
		{#if trigger}{@render trigger()}{:else}{label}{/if}
	</button>
	{#if open}
		<div
			role="menu"
			aria-label={label}
			tabindex="-1"
			onkeydown={handleMenuKeydown}
			class="absolute right-0 z-50 mt-1 min-w-[8rem] rounded-md border border-border bg-popover p-1 text-popover-foreground shadow-md"
		>
			{@render options(close)}
		</div>
	{/if}
</div>
