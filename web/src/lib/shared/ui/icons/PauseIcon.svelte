<script lang="ts">
	/** PauseIcon — pause action affordance. Inherits currentColor. */
	interface Props {
		class?: string;
	}
	let { class: klass = 'h-4 w-4' }: Props = $props();
</script>

<svg
	class={klass}
	viewBox="0 0 24 24"
	fill="currentColor"
	aria-hidden="true"
>
	<rect x="6" y="5" width="4" height="14" rx="1" />
	<rect x="14" y="5" width="4" height="14" rx="1" />
</svg>
