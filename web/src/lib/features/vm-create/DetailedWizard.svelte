<script lang="ts">
	import { tick } from 'svelte';
	import { getVmCreateContext, gabaritCeiling } from './create.svelte';
	import StepBase from './_steps/StepBase.svelte';
	import StepHardware from './_steps/StepHardware.svelte';
	import StepDisk from './_steps/StepDisk.svelte';
	import StepNetwork from './_steps/StepNetwork.svelte';
	import StepReview from './_steps/StepReview.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import { getTaskTrayContext } from '#lib/features/tasks/tasks.svelte.js';
	import { getTaskOutcomeLedgerContext } from '#lib/features/tasks/task-outcome-ledger.svelte.js';
	import { getToastContext } from '#lib/shared/ui/toast.svelte.js';
	import { handleAccepted } from './post-submit';
	import Skeleton from '#lib/shared/ui/Skeleton.svelte';
	import { m } from '#lib/paraglide/messages.js';

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
	type StepState = 'current' | 'done' | 'invalid' | 'todo';

	const HOSTNAME_PATTERN = /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/;

	let current = $state<StepName>('Base');
	let stepContainer: HTMLDivElement | undefined = $state();
	let stepAttempted = $state<Record<StepName, boolean>>({
		Base: false,
		Hardware: false,
		Disk: false,
		Network: false,
		Review: false
	});

	function stepIndex(name: StepName): number {
		return STEPS.findIndex((s) => s.id === name);
	}

	function isStepValid(step: StepName): boolean {
		const cat = form.catalog;
		if (!cat) return false;
		switch (step) {
			case 'Base': {
				const name = form.name.trim();
				if (name === '' || !HOSTNAME_PATTERN.test(name) || form.machineNamed(name) !== null) return false;
				if (form.sourceType !== 'template' && form.node === '') return false;
				if (form.sourceType === 'template' && form.templateId === 0) return false;
				return true;
			}
			case 'Hardware': {
				const maxSockets = gabaritCeiling(cat.gabarit?.maxSockets, 4);
				const maxCores = gabaritCeiling(cat.gabarit?.maxCores, 32);
				const maxMemoryMB = gabaritCeiling(cat.gabarit?.maxMemoryMB, 65536);
				return (
					Number.isInteger(form.sockets) &&
					form.sockets >= 1 &&
					form.sockets <= maxSockets &&
					Number.isInteger(form.cpuCores) &&
					form.cpuCores >= 1 &&
					form.cpuCores <= maxCores &&
					Number.isInteger(form.memoryMB) &&
					form.memoryMB >= 128 &&
					form.memoryMB <= maxMemoryMB
				);
			}
			case 'Disk': {
				if (form.diskStorage === '') return false;
				const maxDiskGB = gabaritCeiling(cat.gabarit?.maxDiskPerVMGB, 2048);
				const minDiskGB = Math.max(1, form.templateMinDiskGB);
				return Number.isInteger(form.diskSizeGB) && form.diskSizeGB >= minDiskGB && form.diskSizeGB <= maxDiskGB;
			}
			case 'Network':
				return form.nics.length > 0 && form.nics.every((nic) => nic.bridge !== '');
			case 'Review':
				return true;
		}
	}

	async function goNext(): Promise<void> {
		const next = STEPS[stepIndex(current) + 1];
		if (next === undefined) return;
		if (!isStepValid(current)) {
			revealInvalid(current);
			await focusFirstInvalid();
			return;
		}
		current = next.id;
	}

	/** Marks a step as attempted so it shows its field errors. */
	function revealInvalid(step: StepName): void {
		stepAttempted = { ...stepAttempted, [step]: true };
		current = step;
	}

	/** Focuses the first control the current step flags as invalid. */
	async function focusFirstInvalid(): Promise<void> {
		await tick();
		const firstInvalid = stepContainer?.querySelector<HTMLElement>('[aria-invalid="true"], input[type="radio"]');
		firstInvalid?.focus();
	}

	/** Step-tab navigation: jumping forward re-checks every step in between
	 *  and stops at the first one that is still incomplete, so a step can
	 *  never be passed (or ticked as done) with missing values. Jumping back
	 *  is always allowed. */
	async function goTo(target: StepName): Promise<void> {
		const targetIndex = stepIndex(target);
		if (targetIndex <= stepIndex(current)) {
			current = target;
			return;
		}
		const blocking = STEPS.slice(0, targetIndex).find((step) => !isStepValid(step.id));
		if (blocking === undefined) {
			current = target;
			return;
		}
		revealInvalid(blocking.id);
		await focusFirstInvalid();
	}

	/** A step earns its check only once its fields hold a valid value - the
	 *  forward gate (goNext/goTo) guarantees this on the way through, but a
	 *  later edit (or a name taken in the meantime) can invalidate one again. */
	function stepState(step: StepName, index: number): StepState {
		if (step === current) return 'current';
		if (index < stepIndex(current)) return isStepValid(step) ? 'done' : 'invalid';
		return 'todo';
	}

	function stepLabel(step: StepName): string {
		return STEPS.find((candidate) => candidate.id === step)?.label() ?? '';
	}

	/** The first step whose fields are still incomplete, or null when the
	 *  form may be submitted (Review itself carries no fields). */
	const blockerStep = $derived(STEPS.slice(0, -1).find((step) => !isStepValid(step.id)) ?? null);

	const stepTabClass: Record<StepState, string> = {
		current: 'text-foreground',
		done: 'text-success-soft-foreground',
		invalid: 'text-destructive-soft-foreground',
		todo: 'text-muted-foreground hover:text-foreground'
	};

	const stepCircleClass: Record<StepState, string> = {
		current: 'border-primary bg-primary-solid text-primary-foreground',
		done: 'border-success-soft-foreground/30 bg-success-soft text-success-soft-foreground',
		invalid: 'border-destructive-soft-border bg-destructive-soft text-destructive-soft-foreground',
		todo: 'border-border bg-muted text-muted-foreground'
	};

	const tray = getTaskTrayContext();
	const toast = getToastContext();
	const outcomeLedger = getTaskOutcomeLedgerContext();

	async function submit(): Promise<void> {
		// Belt and braces: never POST a form with an incomplete step, even if
		// the button is somehow reached with the keyboard.
		if (blockerStep !== null) {
			revealInvalid(blockerStep.id);
			await focusFirstInvalid();
			return;
		}
		const accepted = await form.submit();
		if (accepted === null) {
			if (form.submitError) toast.error(m['toast.vmCreateFailed']({ error: form.submitError }));
			return;
		}
		await handleAccepted(accepted, { tray, toast, outcomeLedger });
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
	<Card as="div" pad="lg">
		{#snippet header()}
			<!-- Progress stepper, not a second tab row: numbered circles joined by a
			     connector, the current step accented, completed steps checked. The
			     role stays `tab` so the step controls keep their existing accessible
			     name and keyboard behaviour. -->
			<ol role="tablist" aria-label={m['vms.create.stepsAriaLabel']()} class="flex flex-wrap items-center gap-1">
				{#each STEPS as step, i (step.id)}
					{@const state = stepState(step.id, i)}
					<li class="flex items-center gap-1">
						<button
							role="tab"
							aria-selected={current === step.id}
							aria-current={current === step.id ? 'step' : undefined}
							class="pv-focus inline-flex items-center gap-2 rounded-lg px-2 py-1.5 text-sm font-medium transition-colors {stepTabClass[
								state
							]}"
							onclick={() => void goTo(step.id)}
						>
							<span
								class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full border text-xs font-semibold transition-colors {stepCircleClass[
									state
								]}"
							>
								{#if state === 'done'}
									<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" class="h-3.5 w-3.5" aria-hidden="true">
										<path d="M4 10l4 4 8-8" />
									</svg>
								{:else if state === 'invalid'}
									<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" class="h-3.5 w-3.5" aria-hidden="true">
										<path d="M10 4.5v6" />
										<path d="M10 15v.01" />
									</svg>
								{:else}
									{i + 1}
								{/if}
							</span>
							{step.label()}
							{#if state === 'invalid'}
								<span class="sr-only">({m['vms.create.stepNeedsFix']()})</span>
							{/if}
						</button>
						{#if i < STEPS.length - 1}
							<span class="h-px w-5 shrink-0 bg-border" aria-hidden="true"></span>
						{/if}
					</li>
				{/each}
			</ol>
		{/snippet}

		<div bind:this={stepContainer}>
			{#if current === 'Base'}
				<StepBase showErrors={stepAttempted.Base} />
			{:else if current === 'Hardware'}
				<StepHardware showErrors={stepAttempted.Hardware} />
			{:else if current === 'Disk'}
				<StepDisk showErrors={stepAttempted.Disk} />
			{:else if current === 'Network'}
				<StepNetwork showErrors={stepAttempted.Network} />
			{:else}
				<StepReview />
			{/if}
		</div>

		{#snippet footer()}
			{#if current === 'Review' && blockerStep !== null}
				<p class="mr-auto text-sm text-destructive-soft-foreground" data-testid="vm-create-blocker">
					{m['vms.create.hintCompleteStep']({ step: stepLabel(blockerStep.id) })}
				</p>
			{/if}
			{#if stepIndex(current) > 0}
				<Button variant="secondary" onclick={goBack}>{m['common.back']()}</Button>
			{/if}
			{#if current === 'Review'}
				<Button
					loading={form.submitting}
					disabled={blockerStep !== null}
					onclick={() => void submit()}
				>
					{form.submitting ? m['common.creating']() : m['vms.create.submit']()}
				</Button>
			{:else}
				<Button onclick={() => void goNext()}>{m['common.next']()}</Button>
			{/if}
		{/snippet}
	</Card>
{/if}
