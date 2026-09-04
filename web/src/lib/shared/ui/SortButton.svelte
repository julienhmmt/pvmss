<script lang="ts">
	/**
	 * SortButton — the clickable label inside a sortable `<th>`.
	 *
	 * Lists were signalling sort direction with a literal " ↑" / " ↓" glyph
	 * appended to the header text, which shifts the column width when the
	 * direction changes and renders at a different weight than the rest of
	 * the header. This reserves the arrow's space permanently and only
	 * changes its opacity and rotation, so the header never reflows: an
	 * inactive column shows a faint arrow on hover, the active column shows
	 * a solid one pointing the sorted way.
	 *
	 * `aria-sort` stays on the `<th>` — the button carries no aria state of
	 * its own beyond its label.
	 */
	interface Props {
		/** Visible column name. */
		label: string;
		/** Whether this column is the current sort key. */
		active: boolean;
		/** Sort direction, when active. */
		direction?: 'asc' | 'desc';
		onclick: () => void;
		[key: string]: unknown;
	}

	let { label, active, direction = 'asc', onclick, ...rest }: Props = $props();
</script>

<button
	type="button"
	class="group -mx-1 inline-flex items-center gap-1 rounded px-1 py-0.5 font-semibold uppercase tracking-[0.04em] transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring {active
		? 'text-foreground'
		: ''}"
	{onclick}
	{...rest}
>
	{label}
	<svg
		viewBox="0 0 12 12"
		class="h-3 w-3 shrink-0 transition-[opacity,transform] duration-150 {active
			? 'text-primary opacity-100'
			: 'opacity-0 group-hover:opacity-40'} {active && direction === 'desc' ? 'rotate-180' : ''}"
		fill="none"
		stroke="currentColor"
		stroke-width="1.75"
		stroke-linecap="round"
		stroke-linejoin="round"
		aria-hidden="true"
	>
		<path d="M6 9.5V2.5" />
		<path d="M3 5.5 6 2.5l3 3" />
	</svg>
</button>
