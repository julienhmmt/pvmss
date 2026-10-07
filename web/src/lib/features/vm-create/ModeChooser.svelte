<script lang="ts">
	/**
	 * ModeChooser - the create screen's first question: Simple or Detailed.
	 *
	 * Two clickable cards that each open a mode's wizard. These are buttons,
	 * not radio cards: the card is a navigation gate ("pick one and go"), and
	 * a radio would dead-click when it is already the selected value.
	 */
	import type { CreateMode } from './create.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Mode {
		id: CreateMode;
		title: () => string;
		description: () => string;
	}

	interface Props {
		/** Fired when the user picks a card; the parent enters that wizard. */
		onSelect: (mode: CreateMode) => void;
	}

	let { onSelect }: Props = $props();

	const MODES: readonly Mode[] = [
		{ id: 'simple', title: m['vms.create.simple'], description: m['vms.create.modeSimpleDescription'] },
		{
			id: 'detailed',
			title: m['vms.create.detailed'],
			description: m['vms.create.modeDetailedDescription']
		}
	];
</script>

<div class="grid gap-3" role="group" aria-labelledby="create-mode-label">
	<p id="create-mode-label" class="text-sm font-medium text-foreground">{m['vms.create.modeLabel']()}</p>
	<div class="grid gap-3 sm:grid-cols-2">
		{#each MODES as mode (mode.id)}
			<button
				type="button"
				onclick={() => onSelect(mode.id)}
				data-testid={`create-mode-${mode.id}`}
				class="pv-focus flex w-full cursor-pointer flex-col gap-1.5 rounded-xl border border-border bg-card p-5 text-left transition-colors hover:border-primary/50 hover:bg-sidebar-accent/40"
			>
				<span class="flex flex-wrap items-center gap-2">
					<span class="text-base font-semibold text-foreground">{mode.title()}</span>
				</span>
				<span class="text-sm text-muted-foreground">{mode.description()}</span>
			</button>
		{/each}
	</div>
</div>
