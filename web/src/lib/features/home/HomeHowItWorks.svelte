<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import StepNumberIcon from '#lib/shared/ui/icons/StepNumberIcon.svelte';
	import { reveal } from '#lib/shared/reveal.js';

	const steps: { number: number; title: () => string; description: () => string }[] = [
		{ number: 1, title: () => m['home.steps.1.title'](), description: () => m['home.steps.1.description']() },
		{ number: 2, title: () => m['home.steps.2.title'](), description: () => m['home.steps.2.description']() },
		{ number: 3, title: () => m['home.steps.3.title'](), description: () => m['home.steps.3.description']() }
	];
</script>

<section class="mx-auto w-full max-w-6xl py-16" aria-labelledby="how-it-works-heading">
	<h2 id="how-it-works-heading" class="text-center text-3xl font-semibold tracking-tight" {@attach reveal()}>
		{m['home.steps.heading']()}
	</h2>
	<div class="relative mt-12" {@attach reveal()}>
		<!-- The connector draws itself across the three steps when they scroll in. -->
		<div class="pv-connector absolute top-6 right-[16.6%] left-[16.6%] hidden h-px bg-primary/40 sm:block" aria-hidden="true"></div>
		<ol class="relative grid gap-10 sm:grid-cols-3 sm:gap-6">
		{#each steps as step, i (step.number)}
			<li class="relative flex flex-col items-center text-center" style:transition-delay="{i * 150}ms" {@attach reveal()}>
				<span class="rounded-full bg-background p-1 ring-8 ring-background">
					<StepNumberIcon number={step.number} class="h-10 w-10" />
				</span>
				<h3 class="mt-4 text-base font-semibold">{step.title()}</h3>
				<p class="mt-1.5 max-w-xs text-sm text-muted-foreground">{step.description()}</p>
			</li>
		{/each}
		</ol>
	</div>
</section>
