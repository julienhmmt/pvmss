<script lang="ts">
	interface Props {
		text: string;
		column: string;
		activeColumn: string;
		sortDir: 'asc' | 'desc';
		onSort: (column: string) => void;
	}

	let { text, column, activeColumn, sortDir, onSort }: Props = $props();

	const isActive = $derived(activeColumn === column);
	const indicator = $derived(isActive ? (sortDir === 'asc' ? ' ↑' : ' ↓') : '');
</script>

<th class="px-4 py-2 font-medium" aria-sort={isActive ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'}>
	<button
		type="button"
		class="inline-flex items-center gap-1 hover:text-primary"
		onclick={() => onSort(column)}
	>
		{text}{indicator}
	</button>
</th>
