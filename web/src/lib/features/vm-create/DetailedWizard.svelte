<script lang="ts">
	import { getVmCreateContext } from './create.svelte';
	import StepBase from './_steps/StepBase.svelte';
	import StepHardware from './_steps/StepHardware.svelte';
	import StepDisk from './_steps/StepDisk.svelte';
	import StepNetwork from './_steps/StepNetwork.svelte';
	import StepReview from './_steps/StepReview.svelte';

	// Detailed-mode wizard (V09): five steps — Base, Hardware, Disk, Network,
	// Review — over the shared create store. Cloud-init is T08's step, not
	// this tranche's (spec Assumptions). Keyboard-navigable: steps are
	// buttons in a tablist, fields are native inputs (constitution XII).
	const form = getVmCreateContext();

	const STEPS = ['Base', 'Hardware', 'Disk', 'Network', 'Review'] as const;
	type StepName = (typeof STEPS)[number];

	let current = $state<StepName>('Base');

	function stepIndex(name: StepName): number {
		return STEPS.indexOf(name);
	}

	function goNext(): void {
		const next = stepIndex(current) + 1;
		const step = STEPS[next];
		if (step !== undefined) current = step;
	}

	function goBack(): void {
		const previous = stepIndex(current) - 1;
		const step = STEPS[previous];
		if (step !== undefined) current = step;
	}
</script>

{#if form.catalog === null}
	<p role="status" aria-live="polite" class="text-muted-foreground">
		{form.catalogError ?? 'Loading catalog…'}
	</p>
{:else}
	<ol role="tablist" aria-label="Creation steps" class="mb-6 flex flex-wrap gap-2">
		{#each STEPS as step (step)}
			<li>
				<button
					role="tab"
					aria-selected={current === step}
					class="rounded-md px-3 py-1.5 text-sm font-medium {current === step
						? 'bg-primary text-primary-foreground'
						: 'bg-muted text-muted-foreground'}"
					onclick={() => (current = step)}
				>
					{step}
				</button>
			</li>
		{/each}
	</ol>

	{#if current === 'Base'}
		<StepBase />
	{:else if current === 'Hardware'}
		<StepHardware />
	{:else if current === 'Disk'}
		<StepDisk />
	{:else if current === 'Network'}
		<StepNetwork />
	{:else}
		<StepReview />
	{/if}

	{#if current !== 'Review'}
		<div class="mt-6 flex gap-2">
			{#if stepIndex(current) > 0}
				<button
					type="button"
					class="rounded-md border border-border px-3 py-2 text-sm font-medium"
					onclick={goBack}
				>
					Back
				</button>
			{/if}
			<button
				type="button"
				class="rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground"
				onclick={goNext}
			>
				Next
			</button>
		</div>
	{/if}
{/if}
