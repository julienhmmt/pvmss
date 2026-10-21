<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import { reveal } from '#lib/shared/reveal.js';

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
		<h2 id="about-architecture-heading" class="text-3xl font-semibold tracking-tight">{m['about.architecture.heading']()}</h2>
		<p class="mt-3 text-muted-foreground">{m['about.architecture.description']()}</p>
	</div>
	<div class="pv-dot-grid mt-10 rounded-2xl border border-border bg-muted/40 px-6 py-10 sm:px-10" {@attach reveal()}>
		<div class="flex flex-col items-center md:flex-row md:items-center">
			{@render node(m['about.architecture.browser'](), m['about.architecture.browserDetail'](), false)}
			{@render connector('0s')}
			<div class="relative w-full max-w-64 md:w-auto md:flex-1">
				<div class="rounded-xl border border-primary/40 bg-card p-5 text-center shadow-[0_0_0_6px] shadow-primary/10">
					<p class="font-semibold tracking-tight text-primary">{m['about.architecture.server']()}</p>
					<p class="mt-1 text-sm text-muted-foreground">{m['about.architecture.serverDetail']()}</p>
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
