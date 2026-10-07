<script lang="ts">
	interface Props {
		class?: string;
	}

	let { class: className = 'h-6 w-6' }: Props = $props();
</script>

<svg
	viewBox="0 0 24 24"
	class={className}
	fill="none"
	stroke="currentColor"
	stroke-width="1.5"
	stroke-linecap="round"
	stroke-linejoin="round"
	aria-hidden="true"
>
	<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
	<polyline points="9 12 12 15 16 10" />
</svg>
