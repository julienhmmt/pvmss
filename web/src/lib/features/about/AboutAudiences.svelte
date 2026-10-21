<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import { CAPABILITIES } from '#lib/features/capabilities/capability-data.js';
	import CapabilityIcon from '#lib/features/capabilities/CapabilityIcon.svelte';
	import FeatureAdminIcon from '#lib/shared/ui/icons/FeatureAdminIcon.svelte';
	import FeatureSelfServiceIcon from '#lib/shared/ui/icons/FeatureSelfServiceIcon.svelte';
	import { reveal } from '#lib/shared/reveal.js';

	const limits = [
		{ title: () => m['capabilities.limits.quota.title'](), description: () => m['capabilities.limits.quota.description']() },
		{ title: () => m['capabilities.limits.gabarit.title'](), description: () => m['capabilities.limits.gabarit.description']() },
		{ title: () => m['capabilities.limits.nodeCapacity.title'](), description: () => m['capabilities.limits.nodeCapacity.description']() }
	];
</script>

<section class="mx-auto w-full max-w-6xl py-14" aria-labelledby="about-audience-heading">
	<h2 id="about-audience-heading" class="text-center text-3xl font-semibold tracking-tight" {@attach reveal()}>
		{m['about.audience.heading']()}
	</h2>
	<div class="mt-10 grid gap-5 lg:grid-cols-2" data-testid="about-audience-grid">
		<article class="flex flex-col rounded-xl border border-border bg-card p-7" {@attach reveal()}>
			<span class="inline-flex w-fit rounded-xl bg-primary/10 p-2.5 text-primary"><FeatureSelfServiceIcon class="h-5 w-5" /></span>
			<h3 class="mt-4 text-xl font-semibold tracking-tight">{m['about.audience.userTitle']()}</h3>
			<p class="mt-2 text-muted-foreground">{m['about.audience.userDescription']()}</p>
			<h4 class="mt-7 text-xs font-semibold tracking-wide text-muted-foreground-subtle uppercase">{m['about.capabilities.heading']()}</h4>
			<ul class="mt-3 space-y-3" data-testid="about-capabilities-list">
				{#each CAPABILITIES as capability (capability.id)}
					<li class="flex items-start gap-3">
						<span class="mt-0.5 text-primary"><CapabilityIcon name={capability.id} class="h-4 w-4" /></span>
						<span class="text-sm">
							<span class="font-medium text-foreground">{capability.title()}</span>
							<span class="text-muted-foreground"> - {capability.description()}</span>
						</span>
					</li>
				{/each}
			</ul>
		</article>
		<article class="flex flex-col rounded-xl border border-border bg-card p-7" style:transition-delay="120ms" {@attach reveal()}>
			<span class="inline-flex w-fit rounded-xl bg-primary/10 p-2.5 text-primary"><FeatureAdminIcon class="h-5 w-5" /></span>
			<h3 class="mt-4 text-xl font-semibold tracking-tight">{m['capabilities.page.adminSectionTitle']()}</h3>
			<p class="mt-2 text-muted-foreground">{m['capabilities.page.adminSectionDescription']()}</p>
			<h4 class="mt-7 text-xs font-semibold tracking-wide text-muted-foreground-subtle uppercase">{m['capabilities.limits.heading']()}</h4>
			<dl class="mt-3 space-y-3" data-testid="about-limits-list">
				{#each limits as limit (limit.title())}
					<div class="rounded-lg border border-border bg-muted/50 px-4 py-3">
						<dt class="text-sm font-medium text-foreground">{limit.title()}</dt>
						<dd class="mt-0.5 text-sm text-muted-foreground">{limit.description()}</dd>
					</div>
				{/each}
			</dl>
		</article>
	</div>
</section>
