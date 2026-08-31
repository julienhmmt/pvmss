<script lang="ts">
	/** DiscIcon — CD-ROM / boot-from-ISO affordance. Inherits currentColor. */
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
	<circle cx="12" cy="12" r="9" />
	<circle cx="12" cy="12" r="2.5" />
</svg>
