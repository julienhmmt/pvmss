<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import { reveal } from '#lib/shared/reveal.js';
	import CinematicBackdrop from '#lib/shared/ui/CinematicBackdrop.svelte';

	const facts = [1, 2, 3, 4, 5].map((n) => () => (m[`about.architecture.fact.${n}` as keyof typeof m] as () => string)());
</script>

{#snippet node(title: string, detail: string, stacked: boolean)}
	<div class="relative w-full max-w-64 md:w-auto md:flex-1">
		{#if stacked}
			<!-- Offset sheets behind the card: "one or more" clusters. -->
			<span class="absolute inset-0 translate-x-2 translate-y-2 rounded-xl border border-border bg-card/60" aria-hidden="true"></span>
			<span class="absolute inset-0 translate-x-1 translate-y-1 rounded-xl border border-border bg-card/80" aria-hidden="true"></span>
		{/if}
		<div class="relative rounded-xl border border-border bg-card p-5 text-center">
			<p class="font-semibold tracking-tight">{title}</p>
			<p class="mt-1 text-sm text-muted-foreground">{detail}</p>
		</div>
	</div>
{/snippet}

{#snippet connector(delay: string)}
	<span class="relative h-10 w-px shrink-0 bg-primary/40 md:h-px md:w-16" aria-hidden="true">
		<span class="pv-flow absolute h-2 w-2 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary" style:animation-delay={delay}></span>
	</span>
{/snippet}

<section class="mx-auto w-full max-w-6xl py-14" aria-labelledby="about-architecture-heading">
	<div class="mx-auto max-w-2xl text-center" {@attach reveal()}>
		<h2 id="about-architecture-heading" class="text-4xl font-semibold tracking-[-0.04em]">{m['about.architecture.heading']()}</h2>
		<p class="mt-3 text-muted-foreground">{m['about.architecture.description']()}</p>
	</div>
	<div class="relative isolate mt-10 overflow-hidden rounded-3xl border border-border bg-card px-6 py-14 sm:px-12" {@attach reveal()}>
		<CinematicBackdrop />
		<div class="flex flex-col items-center justify-center md:flex-row">
			{@render node(m['about.architecture.browser'](), m['about.architecture.browserDetail'](), false)}
			{@render connector('0s')}
			<div class="relative w-full max-w-64 md:w-auto md:flex-1">
				<div class="rounded-2xl bg-primary-solid p-6 text-center text-primary-foreground shadow-[0_0_0_8px] shadow-primary/15">
					<p class="text-lg font-semibold tracking-tight">{m['about.architecture.server']()}</p>
					<p class="mt-1 text-sm text-primary-foreground/80">{m['about.architecture.serverDetail']()}</p>
				</div>
			</div>
			{@render connector('1.4s')}
			{@render node(m['about.architecture.clusters'](), m['about.architecture.clustersDetail'](), true)}
		</div>
		<ul class="mt-10 flex flex-wrap justify-center gap-2">
			{#each facts as fact, i (i)}
				<li class="rounded-full border border-border bg-card px-3 py-1 text-xs font-medium text-muted-foreground">{fact()}</li>
			{/each}
		</ul>
	</div>
</section>
