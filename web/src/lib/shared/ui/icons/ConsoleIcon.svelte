<script lang="ts">
	/** ConsoleIcon — open VM console affordance. Inherits currentColor. */
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
	<rect x="3" y="4" width="18" height="16" rx="2" />
	<path d="M7 9l4 3-4 3" />
	<line x1="14" y1="15" x2="17" y2="15" />
</svg>
