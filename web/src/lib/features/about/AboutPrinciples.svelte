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
		<h2 id="about-principles-heading" class="text-3xl font-semibold tracking-tight lg:sticky lg:top-24" {@attach reveal()}>
			{m['about.principles.heading']()}
		</h2>
	</div>
	<ol class="divide-y divide-border border-y border-border">
		{#each principles as principle (principle.number)}
			<li class="group grid grid-cols-[3.5rem_1fr] gap-4 py-7" {@attach reveal()}>
				<span class="font-mono text-2xl font-medium text-primary/50 transition-colors duration-300 group-hover:text-primary" aria-hidden="true">
					{principle.number}
				</span>
				<div>
					<h3 class="text-lg font-semibold tracking-tight">{principle.title()}</h3>
					<p class="mt-1.5 text-muted-foreground">{principle.description()}</p>
				</div>
			</li>
		{/each}
	</ol>
</section>
