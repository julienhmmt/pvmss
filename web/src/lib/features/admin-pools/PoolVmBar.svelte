<script lang="ts">
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		running: number;
		stopped: number;
		total: number;
	}

	let { running, stopped, total }: Props = $props();

	const runningPercent = $derived(total > 0 ? Math.round((running / total) * 100) : 0);
	const label = $derived(m['admin.pools.vmBarLabel']({ running, stopped, total }));
</script>

{#if total > 0}
	<div
		class="inline-flex flex-wrap items-center gap-2"
		role="meter"
		aria-valuemin={0}
		aria-valuemax={100}
		aria-valuenow={runningPercent}
		aria-label={label}
		title={label}
	>
		<span class="inline-flex items-center gap-1.5 rounded-full border border-success-soft-border bg-success-soft px-2 py-0.5 text-xs font-medium text-success-soft-foreground">
			<span class="h-1.5 w-1.5 rounded-full bg-success"></span>
			{running}
		</span>
		<span class="inline-flex items-center gap-1.5 rounded-full border border-border bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
			<span class="h-1.5 w-1.5 rounded-full bg-muted-foreground"></span>
			{stopped}
		</span>
	</div>
{:else}
	<span class="text-xs text-muted-foreground-subtle">—</span>
{/if}
