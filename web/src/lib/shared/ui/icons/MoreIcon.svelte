<script lang="ts">
	/** MoreIcon - overflow-menu affordance (horizontal ellipsis). */
	interface Props {
		class?: string;
	}
	let { class: klass = 'h-4 w-4' }: Props = $props();
</script>

<svg class={klass} viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
	<circle cx="5" cy="12" r="1.75" />
	<circle cx="12" cy="12" r="1.75" />
	<circle cx="19" cy="12" r="1.75" />
</svg>
