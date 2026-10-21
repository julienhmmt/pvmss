<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import { reveal } from '#lib/shared/reveal.js';

	const before = [() => m['about.why.before.1'](), () => m['about.why.before.2'](), () => m['about.why.before.3'](), () => m['about.why.before.4']()];
	const after = [() => m['about.why.after.1'](), () => m['about.why.after.2'](), () => m['about.why.after.3'](), () => m['about.why.after.4']()];
</script>

<section class="mx-auto w-full max-w-6xl py-14" aria-labelledby="about-why-heading">
	<h2 id="about-why-heading" class="text-center text-3xl font-semibold tracking-tight" {@attach reveal()}>{m['about.why.heading']()}</h2>
	<div class="mt-10 grid gap-5 md:grid-cols-2">
		<div class="rounded-xl border border-dashed border-border bg-muted/50 p-7" {@attach reveal()}>
			<h3 class="text-sm font-semibold tracking-wide text-muted-foreground">{m['about.why.before.title']()}</h3>
			<ol class="relative mt-5 space-y-5 border-l border-dashed border-muted-foreground-subtle/40 pl-6">
				{#each before as item, i (i)}
					<li class="relative text-muted-foreground" style:transition-delay="{i * 120}ms" {@attach reveal()}>
						<span class="absolute top-1.5 -left-[1.95rem] h-3 w-3 rounded-full border border-muted-foreground-subtle/60 bg-muted" aria-hidden="true"></span>
						{item()}
					</li>
				{/each}
			</ol>
		</div>
		<div class="rounded-xl border border-primary/30 bg-card p-7 shadow-[0_1px_2px_rgb(0_0_0/0.04),0_16px_40px_-20px_rgb(0_0_0/0.25)]" {@attach reveal()}>
			<h3 class="text-sm font-semibold tracking-wide text-primary">{m['about.why.after.title']()}</h3>
			<ol class="relative mt-5 space-y-5 border-l border-primary/40 pl-6">
				{#each after as item, i (i)}
					<li class="relative font-medium text-foreground" style:transition-delay="{i * 120}ms" {@attach reveal()}>
						<span class="absolute top-0.5 -left-[2.15rem] grid h-4 w-4 place-items-center rounded-full bg-primary-solid text-primary-foreground" aria-hidden="true">
							<svg viewBox="0 0 16 16" class="h-2.5 w-2.5" fill="none" stroke="currentColor" stroke-width="2.5">
								<path d="M3.5 8.5l3 3 6-7" stroke-linecap="round" stroke-linejoin="round" />
							</svg>
						</span>
						{item()}
					</li>
				{/each}
			</ol>
		</div>
	</div>
</section>
