<script lang="ts">
	interface Props {
		number: number;
		class?: string;
	}

	let { number, class: className = 'h-10 w-10' }: Props = $props();
</script>

<svg
	class="text-primary {className}"
	viewBox="0 0 24 24"
	fill="none"
	aria-hidden="true"
>
	<circle cx="12" cy="12" r="10" fill="currentColor" />
	<text
		x="12"
		y="16.5"
		text-anchor="middle"
		fill="currentColor"
		class="text-primary-foreground"
		font-size="11"
		font-weight="600"
	>{number}</text>
</svg>
