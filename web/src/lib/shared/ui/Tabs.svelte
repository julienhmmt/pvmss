<script lang="ts">
	/**
	 * Tabs - the shared tablist. Two looks for two jobs:
	 *
	 * - `segmented` (default): the pill-in-a-tray form. Right for a small,
	 *   closed set of views inside a card - it reads as a switch.
	 * - `folder`: file-folder tabs attached to the panel below. Right for
	 *   page-level section navigation (VM detail, admin settings): the
	 *   active tab wears the tinted active fill (same as the sidebar's
	 *   current item) and covers the rail, so it reads as the open folder.
	 *   Pair with `.pv-tabpanel` on the tabpanel below: it pulls the first
	 *   card 1px under the rail so the tab and the card share one edge.
	 *
	 * Keyboard behaviour is the same in both: arrow keys move the selection,
	 * Home/End jump to the first/last tab, only the active tab is in the tab
	 * order. Selection is optimistic - the clicked tab renders active at
	 * once, then settles when the bound value catches up.
	 */
	import { m } from '#lib/paraglide/messages.js';
	import type { Component } from 'svelte';

	type Look = 'segmented' | 'folder';

	interface Tab {
		id: string;
		label: () => string;
		/** Optional leading icon (16px, decorative). */
		icon?: Component;
		/** Optional trailing count (e.g. number of snapshots). */
		count?: number;
	}

	interface Props {
		tabs: Tab[];
		active: string;
		look?: Look;
		class?: string;
	}

	let { tabs, active = $bindable(), look = 'segmented', class: klass = '' }: Props = $props();

	// The parent may route the write through the URL (deep links), so the
	// bound value can lag a click. `pending` shows the choice at once and
	// clears when `active` catches up.
	let pending: string | null = $state(null);
	const shown = $derived(pending ?? active);

	$effect(() => {
		active;
		pending = null;
	});

	// Keep the selected tab inside the horizontally scrolling rail.
	$effect(() => {
		document.getElementById(`tab-${shown}`)?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
	});

	function selectTab(id: string): void {
		pending = id;
		active = id;
	}

	// WAI-ARIA tabs pattern: arrows wrap, Home/End jump to the ends.
	function targetIndex(key: string, index: number): number | null {
		switch (key) {
			case 'ArrowRight':
				return (index + 1) % tabs.length;
			case 'ArrowLeft':
				return (index - 1 + tabs.length) % tabs.length;
			case 'Home':
				return 0;
			case 'End':
				return tabs.length - 1;
			default:
				return null;
		}
	}

	function handleKeydown(event: KeyboardEvent, index: number): void {
		const nextIndex = targetIndex(event.key, index);
		if (nextIndex === null) return;
		event.preventDefault();
		const next = tabs[nextIndex];
		if (!next) return;
		selectTab(next.id);
		document.getElementById(`tab-${next.id}`)?.focus();
	}

	const lists: Record<Look, string> = {
		segmented: 'flex flex-wrap gap-1 rounded-xl border border-border bg-muted/50 p-1',
		folder: 'flex gap-1 overflow-x-auto border-b border-border'
	};

	const focusRing = 'pv-focus';

	function tabClass(selected: boolean): string {
		if (look === 'folder') {
			return `relative -mb-px inline-flex h-11 shrink-0 cursor-pointer items-center gap-1.5 rounded-t-xl border px-4 text-sm whitespace-nowrap transition-colors ${focusRing} ${
				selected
					? 'z-10 border-border border-b-transparent bg-sidebar-accent font-semibold text-sidebar-accent-foreground'
					: 'border-transparent font-medium text-muted-foreground hover:bg-sidebar-accent/50 hover:text-foreground'
			}`;
		}
		return `inline-flex min-h-10 cursor-pointer items-center gap-1.5 rounded-[var(--radius-control)] px-3 py-2 text-sm font-medium transition-colors ${focusRing} ${
			selected
				? 'bg-card text-foreground shadow-card'
				: 'text-muted-foreground hover:bg-card/60 hover:text-foreground'
		}`;
	}
</script>

<div role="tablist" class="{lists[look]} {klass}" aria-label={m['common.tabsAriaLabel']()}>
	{#each tabs as tab, index (tab.id)}
		{@const selected = shown === tab.id}
		<button
			type="button"
			id={`tab-${tab.id}`}
			role="tab"
			aria-selected={selected}
			aria-controls={`panel-${tab.id}`}
			tabindex={selected ? 0 : -1}
			class={tabClass(selected)}
			onclick={() => selectTab(tab.id)}
			onkeydown={(event) => handleKeydown(event, index)}
			data-testid={`vm-tab-${tab.id}`}
		>
			{#if tab.icon}
				{@const Icon = tab.icon}
				<Icon class="h-4 w-4 shrink-0" />
			{/if}
			{tab.label()}
			{#if tab.count !== undefined}
				<span
					class="ml-1.5 inline-flex min-w-[1.25rem] justify-center rounded-full px-1.5 py-px font-mono text-2xs tabular-nums {selected
						? 'bg-muted text-foreground'
						: 'bg-muted/70 text-muted-foreground'}"
				>
					{tab.count}
				</span>
			{/if}
		</button>
	{/each}
</div>
