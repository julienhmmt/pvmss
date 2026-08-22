<script lang="ts">
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		running: number;
		stopped: number;
		total: number;
	}

	let { running, stopped, total }: Props = $props();

	const runningPercent = $derived(total > 0 ? Math.round((running / total) * 100) : 0);
	const stoppedPercent = $derived(total > 0 ? Math.round((stopped / total) * 100) : 0);
	const label = $derived(m['admin.pools.vmBarLabel']({ running, stopped, total }));
</script>

{#if total > 0}
	<div class="w-28" role="meter" aria-valuemin={0} aria-valuemax={100} aria-valuenow={runningPercent} aria-label={label}>
		<div class="h-1.5 w-full overflow-hidden rounded-full bg-muted">
			<div class="flex h-full w-full">
				<div
					class="h-full bg-success transition-[width] motion-reduce:transition-none"
					style="width: {runningPercent}%"
				></div>
				<div
					class="h-full bg-muted-foreground transition-[width] motion-reduce:transition-none"
					style="width: {stoppedPercent}%"
				></div>
			</div>
		</div>
	</div>
{:else}
	<span class="text-xs text-muted-foreground-subtle">—</span>
{/if}
