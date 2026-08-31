<script lang="ts">
	/** ResetIcon — hard reset action affordance. Inherits currentColor. */
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
	<path d="M21 12a9 9 0 1 1-2.64-6.36" />
	<path d="M21 3v5h-5" />
	<path d="M12 8v4l2.5 2.5" />
</svg>
