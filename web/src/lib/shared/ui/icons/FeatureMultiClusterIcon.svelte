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
	<circle cx="12" cy="5" r="2.5" />
	<circle cx="6" cy="17" r="2.5" />
	<circle cx="18" cy="17" r="2.5" />
	<line x1="12" y1="7.5" x2="6" y2="14.5" />
	<line x1="12" y1="7.5" x2="18" y2="14.5" />
	<line x1="8.5" y1="17" x2="15.5" y2="17" />
</svg>
