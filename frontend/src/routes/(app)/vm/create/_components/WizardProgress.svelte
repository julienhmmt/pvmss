<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Monitor, Cpu, HardDrive, WifiHigh, Cloud, CheckCircle, Check } from 'phosphor-svelte';
	import type { WizardStep } from '$lib/stores/vm-create.svelte';

	const STEP_ICONS: Record<WizardStep, typeof Monitor> = {
		base: Monitor,
		hardware: Cpu,
		disk: HardDrive,
		network: WifiHigh,
		cloudinit: Cloud,
		review: CheckCircle
	};

	interface Props {
		steps: WizardStep[];
		current: number;
		isStepValid: (step: WizardStep) => boolean;
		onSelect: (index: number) => void;
	}

	let { steps, current, isStepValid, onSelect }: Props = $props();
</script>

<nav class="flex items-center justify-between gap-1 overflow-x-auto pb-2">
	{#each steps as step, i}
		{@const Icon = STEP_ICONS[step]}
		{@const isCurrent = i === current}
		{@const isCompleted = i < current}
		{@const valid = isStepValid(step)}
		<button
			type="button"
			onclick={() => onSelect(i)}
			class="flex min-w-0 flex-1 flex-col items-center gap-1 rounded-lg px-2 py-2 text-xs transition-colors
				{isCurrent
				? 'bg-primary/10 text-primary font-semibold'
				: isCompleted && valid
					? 'text-primary/70 hover:bg-muted'
					: 'text-muted-foreground hover:bg-muted'}"
		>
			<span
				class="flex h-8 w-8 items-center justify-center rounded-full border-2
					{isCurrent
					? 'border-primary bg-primary text-primary-foreground'
					: isCompleted && valid
						? 'border-primary/50 bg-primary/10 text-primary'
						: 'border-muted-foreground/30'}"
			>
				{#if isCompleted && valid}
					<Check class="h-4 w-4" />
				{:else}
					<Icon class="h-4 w-4" />
				{/if}
			</span>
			<span class="truncate">{$t(`vmCreate.steps.${step}`)}</span>
		</button>
		{#if i < steps.length - 1}
			<div
				class="mt-[-1rem] h-px w-4 flex-shrink-0
					{i < current ? 'bg-primary/50' : 'bg-muted-foreground/20'}"
			></div>
		{/if}
	{/each}
</nav>
