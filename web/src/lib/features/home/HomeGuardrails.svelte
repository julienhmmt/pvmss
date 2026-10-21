<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import { reveal } from './reveal';

	// Sample fill levels for the decorative meters - illustrative, not data.
	const limits = [
		{ title: () => m['capabilities.limits.quota.title'](), description: () => m['capabilities.limits.quota.description'](), fill: 'w-3/5' },
		{ title: () => m['capabilities.limits.gabarit.title'](), description: () => m['capabilities.limits.gabarit.description'](), fill: 'w-2/5' },
		{ title: () => m['capabilities.limits.nodeCapacity.title'](), description: () => m['capabilities.limits.nodeCapacity.description'](), fill: 'w-4/5' }
	];
</script>

<section class="w-full py-16" aria-labelledby="home-guardrails-heading">
	<div class="pv-dot-grid mx-auto max-w-6xl rounded-2xl border border-border bg-muted/60 px-6 py-12 sm:px-10">
		<div class="mx-auto max-w-2xl text-center" {@attach reveal()}>
			<h2 id="home-guardrails-heading" class="text-3xl font-semibold tracking-tight">{m['capabilities.limits.heading']()}</h2>
			<p class="mt-3 text-muted-foreground">{m['home.guardrails.description']()}</p>
		</div>
		<ul class="mt-10 grid gap-4 md:grid-cols-3">
			{#each limits as limit, i (i)}
				<li class="rounded-lg border border-border bg-card p-6" style:transition-delay="{i * 120}ms" {@attach reveal()}>
					<h3 class="text-base font-semibold tracking-tight">{limit.title()}</h3>
					<p class="mt-1.5 text-sm text-muted-foreground">{limit.description()}</p>
					<div class="mt-5 h-1.5 overflow-hidden rounded-full bg-muted" aria-hidden="true">
						<div class="pv-load h-full rounded-full bg-primary {limit.fill}" style:animation-duration="{4 + i}s"></div>
					</div>
				</li>
			{/each}
		</ul>
	</div>
</section>
