<script lang="ts" module>
	export type PvHeaderVariant = 'default' | 'danger' | 'warn';
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import { cn } from '$lib/utils.js';

	interface Props {
		title: string;
		eyebrow?: string;
		subtitle?: string;
		variant?: PvHeaderVariant;
		stats?: Snippet;
		actions?: Snippet;
		class?: string;
	}

	let {
		title,
		eyebrow,
		subtitle,
		variant = 'default',
		stats,
		actions,
		class: className
	}: Props = $props();

	const variantClass = $derived(
		variant === 'danger' ? 'pv-header--danger' : variant === 'warn' ? 'pv-header--warn' : ''
	);
</script>

<div class={cn('pv-header -mx-6 -mt-6 mb-6', variantClass, className)}>
	<div class="pv-header-flex">
		<div>
			{#if eyebrow}<p class="pv-eyebrow">{eyebrow}</p>{/if}
			<h1 class="pv-title">{title}</h1>
			{#if subtitle}<p class="pv-subtitle">{subtitle}</p>{/if}
		</div>

		{#if stats || actions}
			<div class="flex items-center gap-3 flex-wrap">
				{#if stats}
					<div class="pv-header-stats">{@render stats()}</div>
				{/if}
				{#if actions}
					{@render actions()}
				{/if}
			</div>
		{/if}
	</div>
</div>
