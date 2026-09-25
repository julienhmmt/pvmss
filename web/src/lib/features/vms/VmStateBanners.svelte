<script lang="ts">
	/**
	 * VmStateBanners - the notices above the detail tabs, in the priority
	 * order of DESIGN.md §6.3: the inline shutdown confirmation, then the
	 * provisioning panel, then failed / partial, then the busy notice. They
	 * are inline, never modal. At most one state banner shows at a time; the
	 * shutdown confirmation can sit above it.
	 */
	import { resolve } from '$app/paths';
	import type { MachineDisplayStatus } from './display-status';
	import Button from '$lib/shared/ui/Button.svelte';
	import ButtonLink from '$lib/shared/ui/ButtonLink.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		status: MachineDisplayStatus;
		/** The user clicked "Shut down" and must confirm. */
		confirmingShutdown: boolean;
		onConfirmShutdown: () => void;
		onCancelShutdown: () => void;
		/** Technical detail recorded with a failed / partial outcome. */
		detail?: string | undefined;
	}

	let { status, confirmingShutdown, onConfirmShutdown, onCancelShutdown, detail }: Props = $props();

	type StepState = 'done' | 'current' | 'pending';
	const steps: { label: () => string; state: StepState }[] = [
		{ label: () => m['vms.detail.banner.stepAccepted'](), state: 'done' },
		{ label: () => m['vms.detail.banner.stepDisk'](), state: 'current' },
		{ label: () => m['vms.detail.banner.stepAccess'](), state: 'pending' },
		{ label: () => m['vms.detail.banner.stepStart'](), state: 'pending' }
	];
	const stepStateLabel: Record<StepState, () => string> = {
		done: () => m['vms.detail.banner.stepDone'](),
		current: () => m['vms.detail.banner.stepCurrent'](),
		pending: () => m['vms.detail.banner.stepPending']()
	};
</script>

{#if confirmingShutdown}
	<div class="rounded-xl border border-warning-soft-border bg-warning-soft p-5 text-warning-soft-foreground" role="alert" data-testid="vm-shutdown-confirmation">
		<p class="font-semibold">{m['vms.detail.banner.shutdownTitle']()}</p>
		<p class="mt-1 text-sm">{m['vms.detail.banner.shutdownBody']()}</p>
		<div class="mt-4 flex flex-wrap gap-2">
			<Button variant="secondary" size="sm" onclick={onCancelShutdown} data-testid="vm-shutdown-cancel">{m['vms.detail.banner.keepRunning']()}</Button>
			<Button size="sm" variant="warning" onclick={onConfirmShutdown} data-testid="vm-shutdown-confirm">
				{m['vms.detail.banner.confirmShutdown']()}
			</Button>
		</div>
	</div>
{/if}

{#if status === 'provisioning'}
	<section class="rounded-xl border border-border bg-card p-6 shadow-card" aria-live="polite" data-testid="vm-banner-provisioning">
		<p class="text-[0.6875rem] font-semibold uppercase tracking-[0.08em] text-primary">{m['vms.detail.banner.provisioningEyebrow']()}</p>
		<p class="mt-1 text-lg font-semibold">{m['vms.detail.banner.provisioningTitle']()}</p>
		<p class="mt-1 text-sm text-muted-foreground">{m['vms.detail.banner.provisioningBody']()}</p>
		<ol class="mt-4 grid gap-2 text-sm">
			{#each steps as step, index (index)}
				<li class="flex items-center gap-3" data-state={step.state}>
					<span
						class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full border text-xs font-semibold {step.state === 'done'
							? 'border-success-soft-border bg-success-soft text-success-soft-foreground'
							: step.state === 'current'
								? 'border-primary bg-sidebar-accent text-sidebar-accent-foreground motion-safe:animate-pulse'
								: 'border-border text-muted-foreground'}"
						aria-hidden="true"
					>
						{step.state === 'done' ? '✓' : index + 1}
					</span>
					<span class={step.state === 'pending' ? 'text-muted-foreground' : 'font-medium'}>{step.label()}</span>
					<span class="sr-only">({stepStateLabel[step.state]()})</span>
				</li>
			{/each}
		</ol>
		<a href={resolve('/vms')} class="pv-focus mt-4 inline-block rounded text-sm font-medium text-primary underline-offset-2 hover:underline">{m['vms.detail.banner.backToWorkspace']()}</a>
	</section>
{:else if status === 'failed' || status === 'partial'}
	<section class="rounded-xl border border-destructive-soft-border bg-destructive-soft p-5 text-destructive-soft-foreground" role="alert" data-testid="vm-banner-{status}">
		<p class="font-semibold">{status === 'failed' ? m['vms.detail.banner.failedTitle']() : m['vms.detail.banner.partialTitle']()}</p>
		<p class="mt-1 text-sm">{status === 'failed' ? m['vms.detail.banner.failedBody']() : m['vms.detail.banner.partialBody']()}</p>
		<div class="mt-3">
			{#if status === 'failed'}
				<ButtonLink href={resolve('/vms/create')} variant="secondary" size="sm">{m['vms.detail.banner.failedAction']()}</ButtonLink>
			{:else}
				<ButtonLink href={resolve('/docs')} variant="secondary" size="sm">{m['vms.detail.banner.partialAction']()}</ButtonLink>
			{/if}
		</div>
		{#if detail}
			<details class="mt-3 text-sm">
				<summary class="cursor-pointer font-medium">{m['vms.detail.banner.technicalDetails']()}</summary>
				<code class="mt-2 block whitespace-pre-wrap break-words rounded-md bg-card/60 p-2 font-mono text-xs">{detail}</code>
			</details>
		{/if}
	</section>
{:else if status === 'starting' || status === 'stopping'}
	<p class="rounded-xl border border-border bg-muted/60 px-5 py-3 text-sm text-muted-foreground" role="status" data-testid="vm-banner-busy">
		{status === 'starting' ? m['vms.detail.banner.busyStarting']() : m['vms.detail.banner.busyStopping']()}
	</p>
{/if}
