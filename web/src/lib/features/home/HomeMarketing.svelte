<script lang="ts">
	import { m } from '$lib/paraglide/messages.js';
	import FeatureSelfServiceIcon from '$lib/shared/ui/icons/FeatureSelfServiceIcon.svelte';
	import FeatureMultiClusterIcon from '$lib/shared/ui/icons/FeatureMultiClusterIcon.svelte';
	import FeatureQuotasIcon from '$lib/shared/ui/icons/FeatureQuotasIcon.svelte';

	/**
	 * HomeMarketing — three static feature cards. Reads no auth state at all
	 * (FR-014: the mechanical proof that marketing content is split from
	 * routing logic). The cards render unchanged regardless of identity.
	 */
</script>

<section class="w-full max-w-4xl grid gap-6 sm:grid-cols-3" aria-label={m['home.features.ariaLabel']()}>
	<div class="rounded-lg border border-border bg-card p-6 text-card-foreground">
		<div class="mb-3 inline-flex rounded-xl bg-primary/10 p-3 text-primary">
			<FeatureSelfServiceIcon class="h-6 w-6" />
		</div>
		<h2 class="text-lg font-semibold tracking-tight">{m['home.feature.1.title']()}</h2>
		<p class="mt-2 text-sm text-muted-foreground">{m['home.feature.1.description']()}</p>
	</div>

	<div class="rounded-lg border border-border bg-card p-6 text-card-foreground">
		<div class="mb-3 inline-flex rounded-xl bg-primary/10 p-3 text-primary">
			<FeatureMultiClusterIcon class="h-6 w-6" />
		</div>
		<h2 class="text-lg font-semibold tracking-tight">{m['home.feature.2.title']()}</h2>
		<p class="mt-2 text-sm text-muted-foreground">{m['home.feature.2.description']()}</p>
	</div>

	<div class="rounded-lg border border-border bg-card p-6 text-card-foreground">
		<div class="mb-3 inline-flex rounded-xl bg-primary/10 p-3 text-primary">
			<FeatureQuotasIcon class="h-6 w-6" />
		</div>
		<h2 class="text-lg font-semibold tracking-tight">{m['home.feature.3.title']()}</h2>
		<p class="mt-2 text-sm text-muted-foreground">{m['home.feature.3.description']()}</p>
	</div>
</section>
