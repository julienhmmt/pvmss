<script lang="ts">
	/**
	 * ModeChooser - the create screen's first question: Simple or Detailed.
	 *
	 * Two clickable cards that each open a mode's wizard. These are buttons,
	 * not radio cards: the card is a navigation gate ("pick one and go"), and
	 * a radio would dead-click when it is already the selected value.
	 */
	import type { CreateMode } from './create.svelte';
	import { m } from '#lib/paraglide/messages.js';

	interface Mode {
		id: CreateMode;
		title: () => string;
		description: () => string;
		bullets: readonly (() => string)[];
		recommended?: boolean;
	}

	interface Props {
		/** Fired when the user picks a card; the parent enters that wizard. */
		onSelect: (mode: CreateMode) => void;
	}

	let { onSelect }: Props = $props();

	const MODES: readonly Mode[] = [
		{
			id: 'simple',
			title: m['vms.create.simple'],
			description: m['vms.create.modeSimpleDescription'],
			bullets: [m['vms.create.modeSimpleBullet1'], m['vms.create.modeSimpleBullet2']],
			recommended: true
		},
		{
			id: 'detailed',
			title: m['vms.create.detailed'],
			description: m['vms.create.modeDetailedDescription'],
			bullets: [m['vms.create.modeDetailedBullet1'], m['vms.create.modeDetailedBullet2']]
		}
	];
</script>

<div class="grid gap-3" role="group" aria-labelledby="create-mode-label">
	<p id="create-mode-label" class="sr-only">{m['vms.create.modeLabel']()}</p>
	<div class="grid gap-3 sm:grid-cols-2">
		{#each MODES as mode (mode.id)}
			<button
				type="button"
				onclick={() => onSelect(mode.id)}
				data-testid={`create-mode-${mode.id}`}
				class={[
					'pv-focus group flex h-full w-full cursor-pointer flex-col gap-3 rounded-xl bg-card p-5 text-left transition-colors hover:border-primary/60 hover:bg-sidebar-accent/40',
					mode.recommended ? 'border border-primary/40' : 'border border-border'
				]}
			>
				<span class="flex items-start justify-between gap-3">
					<span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
						{#if mode.id === 'simple'}
							<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" class="h-5 w-5" aria-hidden="true">
								<path d="M10 2.5l1.6 4.1 4.1 1.6-4.1 1.6L10 14l-1.6-4.2-4.1-1.6 4.1-1.6L10 2.5z" />
								<path d="M15.5 13.5l.7 1.8 1.8.7-1.8.7-.7 1.8-.7-1.8-1.8-.7 1.8-.7.7-1.8z" />
							</svg>
						{:else}
							<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" class="h-5 w-5" aria-hidden="true">
								<path d="M3 5h9M16 5h1M3 10h3M10 10h7M3 15h11M18 15h-1" />
								<circle cx="14" cy="5" r="2" />
								<circle cx="8" cy="10" r="2" />
								<circle cx="15" cy="15" r="2" />
							</svg>
						{/if}
					</span>
					{#if mode.recommended}
						<span class="rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary">{m['vms.create.recommended']()}</span>
					{/if}
				</span>
				<span class="text-base font-semibold text-foreground">{mode.title()}</span>
				<span class="text-sm text-muted-foreground">{mode.description()}</span>
				<ul class="flex flex-col gap-1.5">
					{#each mode.bullets as bullet (bullet)}
						<li class="flex items-start gap-2 text-sm text-foreground/80">
							<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="mt-0.5 h-4 w-4 shrink-0 text-primary" aria-hidden="true">
								<path d="M4.5 10.5l3.5 3.5 7.5-8" />
							</svg>
							<span>{bullet()}</span>
						</li>
					{/each}
				</ul>
				<span class="mt-auto flex justify-end">
					<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-4 w-4 text-muted-foreground transition-transform group-hover:translate-x-0.5" aria-hidden="true">
						<path d="M8 5l5 5-5 5" />
					</svg>
				</span>
			</button>
		{/each}
	</div>
</div>
