<script lang="ts">
	import Tooltip from './Tooltip.svelte';
	import InfoIcon from './icons/InfoIcon.svelte';

	interface Props {
		text: string;
		tooltip?: string;
		column?: string;
		activeColumn?: string;
		sortDir?: 'asc' | 'desc';
		onSort?: (column: string) => void;
	}

	let { text, tooltip, column = '', activeColumn = '', sortDir = 'asc', onSort }: Props = $props();

	const isActive = $derived(column !== '' && activeColumn === column);
	const indicator = $derived(isActive ? (sortDir === 'asc' ? ' ↑' : ' ↓') : '');
	const ariaSort = $derived(isActive ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none');
	const sortable = $derived(column !== '' && onSort !== undefined);
</script>

<th class="px-4 py-2 font-medium" aria-sort={sortable ? ariaSort : undefined}>
	<span class="inline-flex items-center gap-1">
		{#if sortable}
			<button
				type="button"
				class="inline-flex items-center gap-1 hover:text-primary"
				onclick={() => onSort?.(column)}
			>
				{text}{indicator}
			</button>
		{:else}
			{text}
		{/if}
		{#if tooltip}
			<Tooltip text={tooltip}>
				<InfoIcon class="h-3.5 w-3.5 text-muted-foreground" />
			</Tooltip>
		{/if}
	</span>
</th>
