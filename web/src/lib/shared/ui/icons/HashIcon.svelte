<script lang="ts">
	/** HashIcon - number sign, used for the VM ID range card on the policy page. Inherits currentColor. */
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
	<line x1="4" x2="20" y1="9" y2="9" />
	<line x1="4" x2="20" y1="15" y2="15" />
	<line x1="10" x2="8" y1="3" y2="21" />
	<line x1="16" x2="14" y1="3" y2="21" />
</svg>
