<script lang="ts">
	/** StopIcon — force-stop action affordance. Inherits currentColor. */
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
	<rect x="6" y="6" width="12" height="12" rx="1.5" />
</svg>
