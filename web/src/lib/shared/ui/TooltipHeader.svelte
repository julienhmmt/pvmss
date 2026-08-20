<script lang="ts">
	import InfoIcon from '$lib/shared/ui/icons/InfoIcon.svelte';

	interface Props {
		text: string;
		tooltip: string;
	}

	let { text, tooltip }: Props = $props();
</script>

<th class="px-4 py-2 font-medium" title={tooltip}>
	<span class="inline-flex items-center gap-1">
		{text}
		<InfoIcon class="h-3.5 w-3.5 text-muted-foreground" />
	</span>
</th>
