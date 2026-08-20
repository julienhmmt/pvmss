<script lang="ts">
	import Tooltip from '$lib/shared/ui/Tooltip.svelte';
	import InfoIcon from '$lib/shared/ui/icons/InfoIcon.svelte';

	interface Props {
		text: string;
		tooltip: string;
		column: string;
		activeColumn: string;
		sortDir: 'asc' | 'desc';
		onSort: (column: string) => void;
	}

	let { text, tooltip, column, activeColumn, sortDir, onSort }: Props = $props();

	const isActive = $derived(activeColumn === column);
	const indicator = $derived(isActive ? (sortDir === 'asc' ? ' ↑' : ' ↓') : '');
</script>

<th class="px-4 py-2 font-medium" aria-sort={isActive ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'}>
	<span class="inline-flex items-center gap-1">
		<button
			type="button"
			class="inline-flex items-center gap-1 hover:text-primary"
			onclick={() => onSort(column)}
		>
			{text}{indicator}
		</button>
		<Tooltip text={tooltip}>
			<InfoIcon class="h-3.5 w-3.5 text-muted-foreground" />
		</Tooltip>
	</span>
</th>
