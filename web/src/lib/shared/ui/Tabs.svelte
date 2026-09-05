<script lang="ts">
	/**
	 * Tabs — the shared tablist. Two looks for two jobs:
	 *
	 * - `segmented` (default): the pill-in-a-tray form. Right for a small,
	 *   closed set of views inside a card — it reads as a switch.
	 * - `underline`: a rule with the active tab underscored in the accent.
	 *   Right for page-level section navigation (VM detail: Hardware, Disks,
	 *   Network, Snapshots, …), where a tray of pills competes with the page
	 *   header for attention and stops scaling past four or five entries.
	 *
	 * Keyboard behaviour is the same in both: arrow keys move the selection,
	 * only the active tab is in the tab order.
	 */
	import { m } from '$lib/paraglide/messages.js';

	type Look = 'segmented' | 'underline';

	interface Tab {
		id: string;
		label: () => string;
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

	function selectTab(id: string): void {
		active = id;
	}

	function handleKeydown(event: KeyboardEvent, index: number): void {
		if (event.key !== 'ArrowRight' && event.key !== 'ArrowLeft') return;
		event.preventDefault();
		const delta = event.key === 'ArrowRight' ? 1 : -1;
		const nextIndex = (index + delta + tabs.length) % tabs.length;
		const next = tabs[nextIndex];
		if (!next) return;
		active = next.id;
		document.getElementById(`tab-${next.id}`)?.focus();
	}

	const lists: Record<Look, string> = {
		segmented: 'flex flex-wrap gap-1 rounded-xl border border-border bg-muted/50 p-1',
		underline: 'flex gap-1 overflow-x-auto border-b border-border'
	};

	const focusRing =
		'pv-focus';

	function tabClass(selected: boolean): string {
		if (look === 'underline') {
			return `relative shrink-0 whitespace-nowrap border-b-2 px-3 py-2.5 text-sm font-medium transition-colors -mb-px ${focusRing} ${
				selected
					? 'border-primary text-foreground'
					: 'border-transparent text-muted-foreground hover:border-border hover:text-foreground'
			}`;
		}
		return `rounded-[var(--radius-control)] px-3 py-2 text-sm font-medium transition-colors ${focusRing} ${
			selected
				? 'bg-card text-foreground shadow-card'
				: 'text-muted-foreground hover:bg-card/60 hover:text-foreground'
		}`;
	}
</script>

<div role="tablist" class="{lists[look]} {klass}" aria-label={m['common.tabsAriaLabel']()}>
	{#each tabs as tab, index (tab.id)}
		{@const selected = active === tab.id}
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
			{tab.label()}
			{#if tab.count !== undefined}
				<span
					class="ml-1.5 inline-flex min-w-[1.25rem] justify-center rounded-full px-1.5 py-px font-mono text-[0.6875rem] tabular-nums {selected
						? 'bg-muted text-foreground'
						: 'bg-muted/70 text-muted-foreground'}"
				>
					{tab.count}
				</span>
			{/if}
		</button>
	{/each}
</div>
