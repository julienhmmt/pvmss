<script lang="ts">
	/** LockIcon — password affordance for input leading icons. Inherits currentColor. */
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
	<rect x="5" y="11" width="14" height="10" rx="2" />
	<path d="M8 11V7a4 4 0 0 1 8 0v4" />
</svg>
