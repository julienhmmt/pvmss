<script lang="ts">
	/** GaugeIcon - speedometer dial, used for the VM Overview tab. Inherits currentColor. */
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
	<path d="M4 14a8 8 0 1 1 16 0" />
	<path d="M12 14l4-4" />
	<circle cx="12" cy="14" r="1" fill="currentColor" stroke="none" />
</svg>
