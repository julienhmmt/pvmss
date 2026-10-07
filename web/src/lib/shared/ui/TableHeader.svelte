<script lang="ts" generics="C extends string">
	/**
	 * TableHeader - a `<th>` for the admin tables, with optional sorting and
	 * an optional tooltip.
	 *
	 * `C` binds `column` to the caller's sort-column union: a table whose
	 * columns are `NodeSortColumn` can hand `onSort` straight through and
	 * stay type-checked - no `as` casts at the call site.
	 *
	 * The cell's own look (padding, uppercase, sticky band) comes from
	 * `.pv-table thead th` in app.css, so this component no longer carries
	 * spacing utilities of its own - that is what let admin tables drift
	 * apart from the VM list in the first place. Sorting is signalled by the
	 * shared SortButton (a reserved-width arrow) rather than by appending a
	 * ↑/↓ glyph to the label, which shifted the column on every sort.
	 */
	import Tooltip from './Tooltip.svelte';
	import InfoIcon from './icons/InfoIcon.svelte';
	import SortButton from './SortButton.svelte';

	interface Props {
		text: string;
		tooltip?: string;
		column?: C;
		activeColumn?: string;
		sortDir?: 'asc' | 'desc';
		onSort?: (column: C) => void;
		/** Right-align the column - for figures and action columns. */
		numeric?: boolean;
		class?: string;
	}

	let {
		text,
		tooltip,
		column,
		activeColumn = '',
		sortDir = 'asc',
		onSort,
		numeric = false,
		class: className = ''
	}: Props = $props();

	const hasColumn = $derived(column !== undefined && column !== '');
	const isActive = $derived(hasColumn && activeColumn === column);
	const ariaSort = $derived(isActive ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none');
	const sortable = $derived(hasColumn && onSort !== undefined);
</script>

<th class="{numeric ? 'num' : ''} {className}" aria-sort={sortable ? ariaSort : undefined}>
	<span class="inline-flex items-center gap-1">
		{#if sortable && column}
			<SortButton
				label={text}
				active={isActive}
				direction={sortDir}
				onclick={() => onSort?.(column)}
			/>
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
