<script lang="ts">
	interface Props {
		class?: string;
	}

	let { class: className = 'h-4 w-4' }: Props = $props();
</script>

<svg
	viewBox="0 0 24 24"
	class={className}
	fill="none"
	stroke="currentColor"
	stroke-width="2"
	aria-hidden="true"
>
	<line x1="6" y1="6" x2="18" y2="18" />
	<line x1="18" y1="6" x2="6" y2="18" />
</svg>
