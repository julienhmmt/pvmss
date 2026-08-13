<script lang="ts">
	import { m } from '$lib/paraglide/messages.js';

	/**
	 * HomeMarketing — three static feature cards. Reads no auth state at all
	 * (FR-014: the mechanical proof that marketing content is split from
	 * routing logic). The cards render unchanged regardless of identity.
	 */
	const features: { title: () => string; description: () => string }[] = [
		{ title: () => m['home.feature.1.title'](), description: () => m['home.feature.1.description']() },
		{ title: () => m['home.feature.2.title'](), description: () => m['home.feature.2.description']() },
		{ title: () => m['home.feature.3.title'](), description: () => m['home.feature.3.description']() }
	];
</script>

<section class="grid gap-6 sm:grid-cols-3" aria-label="Features">
	{#each features as feature (feature.title())}
		<div class="rounded-lg border border-border bg-card p-6 text-card-foreground">
			<h2 class="text-lg font-semibold tracking-tight">{feature.title()}</h2>
			<p class="mt-2 text-sm text-muted-foreground">{feature.description()}</p>
		</div>
	{/each}
</section>
