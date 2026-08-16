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
	<rect x="4" y="2" width="16" height="8" rx="2" />
	<rect x="4" y="13" width="16" height="8" rx="2" />
	<circle cx="7.5" cy="6" r="1" fill="currentColor" />
	<circle cx="7.5" cy="17" r="1" fill="currentColor" />
	<line x1="11" y1="6" x2="18" y2="6" />
	<line x1="11" y1="17" x2="18" y2="17" />
</svg>
