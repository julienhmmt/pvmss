<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import { reveal } from '#lib/shared/reveal.js';

	// The five product principles of PRODUCT.md, in the same order.
	const principles = [1, 2, 3, 4, 5].map((n) => ({
		number: String(n).padStart(2, '0'),
		title: () => (m[`about.principles.${n}.title` as keyof typeof m] as () => string)(),
		description: () => (m[`about.principles.${n}.description` as keyof typeof m] as () => string)()
	}));
</script>

<section class="mx-auto grid w-full max-w-6xl gap-10 py-14 lg:grid-cols-[1fr_2fr]" aria-labelledby="about-principles-heading">
	<div>
		<h2 id="about-principles-heading" class="text-4xl leading-[1.05] font-semibold tracking-[-0.04em] text-balance lg:sticky lg:top-24" {@attach reveal()}>
			{m['about.principles.heading']()}
		</h2>
	</div>
	<ol class="divide-y divide-border border-y border-border">
		{#each principles as principle (principle.number)}
			<li class="group grid grid-cols-[5.5rem_1fr] items-baseline gap-4 py-8" {@attach reveal()}>
				<span class="about-num text-6xl leading-none font-semibold tracking-[-0.06em] text-transparent transition-colors duration-300 group-hover:text-primary" aria-hidden="true">
					{principle.number}
				</span>
				<div>
					<h3 class="text-xl font-semibold tracking-[-0.025em]">{principle.title()}</h3>
					<p class="mt-1.5 text-muted-foreground">{principle.description()}</p>
				</div>
			</li>
		{/each}
	</ol>
</section>

<style>
	/* Outlined numerals that fill with the accent on hover. */
	.about-num {
		-webkit-text-stroke: 1.5px var(--primary);
	}
</style>
