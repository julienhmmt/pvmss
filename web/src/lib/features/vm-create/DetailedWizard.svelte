<script lang="ts">
	import { tick } from 'svelte';
	import { getVmCreateContext, gabaritCeiling } from './create.svelte';
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
				if (form.sourceType === 'image') {
					if (form.imageFile === '' || form.ciUser.trim() === '' || form.sshKeys().length === 0) return false;
					if (form.ciIpMode === 'static' && form.ciIpAddress.trim() === '') return false;
				}
				return true;
			}
			case 'Hardware': {
				if (form.sourceType === 'image' && form.hasProfiles()) {
					return form.profileId !== '' && cat.profiles.some((p) => p.id === form.profileId);
				}
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
				if (form.sourceType === 'image' && form.hasProfiles()) return true;
				const maxDiskGB = gabaritCeiling(cat.gabarit?.maxDiskPerVMGB, 2048);
				const minDiskGB = Math.max(1, form.templateMinDiskGB, form.sourceType === 'image' ? form.imageMinDiskGB : 0);
				return Number.isInteger(form.diskSizeGB) && form.diskSizeGB >= minDiskGB && form.diskSizeGB <= maxDiskGB;
			}
			case 'Network':
				return form.nics.length > 0 && form.nics.every((nic) => nic.bridge !== '');
			case 'Review':
				return true;
		}
	}

	async function goNext(): Promise<void> {
		if (!isStepValid(current)) {
			stepAttempted = { ...stepAttempted, [current]: true };
			await tick();
			const firstInvalid = stepContainer?.querySelector<HTMLElement>('[aria-invalid="true"], input[type="radio"]');
			firstInvalid?.focus();
			return;
		}
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

	{#if current !== 'Review'}
		<div class="mt-6 flex gap-2">
			{#if stepIndex(current) > 0}
				<Button variant="secondary" onclick={goBack}>{m['common.back']()}</Button>
			{/if}
			<Button onclick={() => void goNext()}>{m['common.next']()}</Button>
		</div>
	{/if}
{/if}
