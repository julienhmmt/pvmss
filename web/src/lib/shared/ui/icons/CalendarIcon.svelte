<script lang="ts">
	/** CalendarIcon - calendar, used for the Leases card on the policy page. Inherits currentColor. */
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
	<path d="M8 2v4M16 2v4" />
	<rect width="18" height="18" x="3" y="4" rx="2" />
	<path d="M3 10h18" />
</svg>
