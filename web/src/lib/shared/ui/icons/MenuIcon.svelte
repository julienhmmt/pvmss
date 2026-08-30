<script lang="ts">
	interface Props {
		class?: string;
	}

	let { class: className = 'h-5 w-5' }: Props = $props();
</script>

<svg
	viewBox="0 0 24 24"
	class={className}
	fill="none"
	stroke="currentColor"
	stroke-width="2"
	aria-hidden="true"
>
	<line x1="4" y1="7" x2="20" y2="7" />
	<line x1="4" y1="12" x2="20" y2="12" />
	<line x1="4" y1="17" x2="20" y2="17" />
</svg>
