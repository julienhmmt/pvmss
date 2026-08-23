<script lang="ts">
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';
	import { CAPABILITIES } from '$lib/features/capabilities/capability-data';
	import CapabilityIcon from '$lib/features/capabilities/CapabilityIcon.svelte';
	import FeatureCard from '$lib/features/capabilities/FeatureCard.svelte';
	import FeatureAdminIcon from '$lib/shared/ui/icons/FeatureAdminIcon.svelte';
	import FeatureMultiClusterIcon from '$lib/shared/ui/icons/FeatureMultiClusterIcon.svelte';
	import FeatureQuotasIcon from '$lib/shared/ui/icons/FeatureQuotasIcon.svelte';
	import FeatureSelfServiceIcon from '$lib/shared/ui/icons/FeatureSelfServiceIcon.svelte';

	/**
	 * HomeCapabilities — combined product feature + capability grid.
	 * Renders for everyone (no auth state). The first 3 cards are the
	 * high-level feature highlights; the next 5 are the unique capability
	 * cards; the 9th is the admin section preview. The duplicate
	 * "Multi-cluster" card is shown once via the feature highlight.
	 */
	const coreCapabilities = CAPABILITIES.filter((capability) => capability.id !== 'multiCluster');
</script>

<section class="w-full max-w-4xl" aria-labelledby="home-capabilities-heading">
	<h2 id="home-capabilities-heading" class="text-center text-xl font-semibold tracking-tight">
		{m['capabilities.heading']()}
	</h2>
	<p class="mt-2 text-center text-sm text-muted-foreground">{m['capabilities.subtitle']()}</p>

	<div class="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3" data-testid="home-capabilities-grid">
		<FeatureCard title={() => m['home.feature.1.title']()} description={() => m['home.feature.1.description']()} headingLevel="h3">
			{#snippet icon()}
				<FeatureSelfServiceIcon class="h-5 w-5" />
			{/snippet}
		</FeatureCard>

		<FeatureCard title={() => m['home.feature.2.title']()} description={() => m['home.feature.2.description']()} headingLevel="h3">
			{#snippet icon()}
				<FeatureMultiClusterIcon class="h-5 w-5" />
			{/snippet}
		</FeatureCard>

		<FeatureCard title={() => m['home.feature.3.title']()} description={() => m['home.feature.3.description']()} headingLevel="h3">
			{#snippet icon()}
				<FeatureQuotasIcon class="h-5 w-5" />
			{/snippet}
		</FeatureCard>

		{#each coreCapabilities as capability (capability.id)}
			<FeatureCard title={capability.title} description={capability.description} headingLevel="h3">
				{#snippet icon()}
					<CapabilityIcon name={capability.id} class="h-5 w-5" />
				{/snippet}
			</FeatureCard>
		{/each}

		<FeatureCard
			title={() => m['capabilities.page.adminSectionTitle']()}
			description={() => m['capabilities.page.adminSectionDescription']()}
			headingLevel="h3"
		>
			{#snippet icon()}
				<FeatureAdminIcon class="h-5 w-5" />
			{/snippet}
		</FeatureCard>
	</div>

	<div class="mt-5 flex flex-col items-center gap-2">
		<p class="text-sm text-muted-foreground">{m['capabilities.limits.heading']()}</p>
		<a
			href={resolve('/about')}
			class="rounded-md border border-border px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-accent"
			data-testid="home-capabilities-about-link"
		>
			{m['capabilities.aboutTitle']()}
		</a>
	</div>
</section>
