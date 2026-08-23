<script lang="ts">
	import type { Capability } from './capability-data';
	import CapabilityIcon from './CapabilityIcon.svelte';
	import FeatureCard from './FeatureCard.svelte';

	interface Props {
		capability: Capability;
		headingLevel?: 'h2' | 'h3';
	}

	let { capability, headingLevel = 'h3' }: Props = $props();
</script>

<FeatureCard title={capability.title} description={capability.description} {headingLevel}>
	{#snippet icon()}
		<CapabilityIcon name={capability.id} class="h-5 w-5" />
	{/snippet}
</FeatureCard>
