<script lang="ts">
	import { getVmCreateContext } from './create.svelte';
	import StepBase from './_steps/StepBase.svelte';
	import StepHardware from './_steps/StepHardware.svelte';
	import StepDisk from './_steps/StepDisk.svelte';
	import StepNetwork from './_steps/StepNetwork.svelte';
	import StepReview from './_steps/StepReview.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';
	import { m } from '$lib/paraglide/messages.js';

	// Detailed-mode wizard (V09): five steps - Base, Hardware, Disk, Network,
	// Review - over the shared create store. Cloud-init is T08's step, not
	// this tranche's (spec Assumptions). Keyboard-navigable: steps are
	// buttons in a tablist, fields are native inputs (constitution XII).
	const form = getVmCreateContext();

	const STEPS = [
		{ id: 'Base', label: () => m['vms.create.stepBase']() },
		{ id: 'Hardware', label: () => m['vms.create.stepHardware']() },
		{ id: 'Disk', label: () => m['vms.create.stepDisk']() },
		{ id: 'Network', label: () => m['vms.create.stepNetwork']() },
		{ id: 'Review', label: () => m['vms.create.stepReview']() }
	] as const;
	type StepName = (typeof STEPS)[number]['id'];

	let current = $state<StepName>('Base');

	function stepIndex(name: StepName): number {
		return STEPS.findIndex((s) => s.id === name);
	}

	function goNext(): void {
		const next = stepIndex(current) + 1;
		const step = STEPS[next];
		if (step !== undefined) current = step.id;
	}

	function goBack(): void {
		const previous = stepIndex(current) - 1;
		const step = STEPS[previous];
		if (step !== undefined) current = step.id;
	}
</script>

{#if form.catalog === null}
	<div class="grid gap-3" role="status" aria-live="polite">
		<Skeleton class="h-8 w-full" />
		<Skeleton class="h-4 w-24" />
		<Skeleton class="h-10 w-full" />
		<Skeleton class="h-4 w-20" />
		<Skeleton class="h-10 w-full" />
	</div>
{:else}
	<!-- Progress stepper, not a second tab row: numbered circles joined by a
	     connector, the current step accented, completed steps checked. The
	     role stays `tab` so the step controls keep their existing accessible
	     name and keyboard behaviour. -->
	<ol role="tablist" aria-label={m['vms.create.stepsAriaLabel']()} class="mb-6 flex flex-wrap items-center gap-1">
		{#each STEPS as step, i (step.id)}
			{@const state = current === step.id ? 'current' : stepIndex(current) > i ? 'done' : 'todo'}
			<li class="flex items-center gap-1">
				<button
					role="tab"
					aria-selected={current === step.id}
					aria-current={current === step.id ? 'step' : undefined}
					class="pv-focus inline-flex items-center gap-2 rounded-lg px-2 py-1.5 text-sm font-medium transition-colors {state === 'current'
						? 'text-foreground'
						: state === 'done'
							? 'text-success-soft-foreground'
							: 'text-muted-foreground hover:text-foreground'}"
					onclick={() => (current = step.id)}
				>
					<span
						class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full border text-xs font-semibold transition-colors {state === 'current'
							? 'border-primary bg-primary-solid text-primary-foreground'
							: state === 'done'
								? 'border-success-soft-foreground/30 bg-success-soft text-success-soft-foreground'
								: 'border-border bg-muted text-muted-foreground'}"
					>
						{#if state === 'done'}
							<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" class="h-3.5 w-3.5" aria-hidden="true">
								<path d="M4 10l4 4 8-8" />
							</svg>
						{:else}
							{i + 1}
						{/if}
					</span>
					{step.label()}
				</button>
				{#if i < STEPS.length - 1}
					<span class="h-px w-5 shrink-0 bg-border" aria-hidden="true"></span>
				{/if}
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
				<Button variant="secondary" onclick={goBack}>{m['common.back']()}</Button>
			{/if}
			<Button onclick={goNext}>{m['common.next']()}</Button>
		</div>
	{/if}
{/if}
