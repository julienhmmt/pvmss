<script lang="ts">
	interface Props {
		current: number;
		max: number;
		label: string;
		widthClass?: string;
	}

	let { current, max, label, widthClass = 'w-24' }: Props = $props();

	const pct = $derived(max > 0 ? Math.round((current / max) * 100) : 0);
</script>

{#if max > 0}
	<div class="pv-usage-bar {widthClass}">
		<div class="pv-usage-bar-track" style="flex:1">
			<div class="pv-usage-bar-fill" style="width:{pct}%"></div>
		</div>
		<span class="pv-usage-label">{label}</span>
	</div>
{:else}
	<span class="text-muted-foreground">—</span>
{/if}
