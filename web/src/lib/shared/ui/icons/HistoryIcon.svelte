<script lang="ts">
	/** HistoryIcon - clock with a rewind arrow, used for the VM Snapshots tab. Inherits currentColor. */
	interface Props {
		class?: string;
	}
	let { class: klass = 'h-4 w-4' }: Props = $props();
</script>

<svg
	class={klass}
	viewBox="0 0 24 24"
	fill="none"
	stroke="currentColor"
	stroke-width="1.75"
	stroke-linecap="round"
	stroke-linejoin="round"
	aria-hidden="true"
>
	<path d="M3 12a9 9 0 1 0 3-6.7L3 8" />
	<path d="M3 3v5h5" />
	<path d="M12 7v5l3.5 2" />
</svg>
