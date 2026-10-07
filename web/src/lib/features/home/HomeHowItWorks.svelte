<script lang="ts">
	import { m } from '$lib/paraglide/messages.js';
	import StepNumberIcon from '$lib/shared/ui/icons/StepNumberIcon.svelte';
	import { getSessionContext } from '$lib/features/auth/session.svelte';

	const session = getSessionContext();

	const steps: { number: number; title: () => string; description: () => string }[] = [
		{ number: 1, title: () => m['home.steps.1.title'](), description: () => m['home.steps.1.description']() },
		{ number: 2, title: () => m['home.steps.2.title'](), description: () => m['home.steps.2.description']() },
		{ number: 3, title: () => m['home.steps.3.title'](), description: () => m['home.steps.3.description']() }
	];
</script>

{#if !session.principal}
	<section class="w-full max-w-4xl" aria-labelledby="how-it-works-heading">
		<h2 id="how-it-works-heading" class="text-center text-xl font-semibold tracking-tight">
			{m['home.steps.heading']()}
		</h2>
		<div class="mt-6 grid gap-6 sm:grid-cols-3">
			{#each steps as step (step.number)}
				<div class="flex flex-col items-center text-center">
					<StepNumberIcon number={step.number} class="h-10 w-10" />
					<h3 class="mt-3 text-base font-semibold">{step.title()}</h3>
					<p class="mt-1 text-sm text-muted-foreground">{step.description()}</p>
				</div>
			{/each}
		</div>
	</section>
{/if}
