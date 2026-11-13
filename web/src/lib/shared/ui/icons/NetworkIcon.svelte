<script lang="ts">
	/** NetworkIcon - connected nodes, used for the Network group on the policy page. Inherits currentColor. */
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
	<rect x="16" y="16" width="6" height="6" rx="1" />
	<rect x="2" y="16" width="6" height="6" rx="1" />
	<rect x="9" y="2" width="6" height="6" rx="1" />
	<path d="M5 16v-3a1 1 0 0 1 1-1h12a1 1 0 0 1 1 1v3" />
	<path d="M12 12V8" />
</svg>
