<script lang="ts">
	/** UserIcon — user/identity affordance for input leading icons. Inherits currentColor. */
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
	<circle cx="12" cy="8" r="4" />
	<path d="M4 20c0-4 4-6 8-6s8 2 8 6" />
</svg>
