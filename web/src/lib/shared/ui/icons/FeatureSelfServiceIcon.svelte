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
	<rect x="3" y="4" width="18" height="12" rx="2" />
	<line x1="8" y1="20" x2="16" y2="20" />
	<line x1="12" y1="16" x2="12" y2="20" />
	<polyline points="8 10 11 13 16 8" />
</svg>
