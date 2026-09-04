<script lang="ts">
	import { getVmDetailContext, type VmAction } from './detail.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import PlayIcon from '$lib/shared/ui/icons/PlayIcon.svelte';
	import PowerOffIcon from '$lib/shared/ui/icons/PowerOffIcon.svelte';
	import StopIcon from '$lib/shared/ui/icons/StopIcon.svelte';
	import RestartIcon from '$lib/shared/ui/icons/RestartIcon.svelte';
	import ResetIcon from '$lib/shared/ui/icons/ResetIcon.svelte';
	import PauseIcon from '$lib/shared/ui/icons/PauseIcon.svelte';
	import TrashIcon from '$lib/shared/ui/icons/TrashIcon.svelte';
	import type { Component } from 'svelte';
	import Button from '$lib/shared/ui/Button.svelte';

	const store = getVmDetailContext();
	const toast = getToastContext();

	type ActionDef = {
		kind: VmAction;
		label: () => string;
		icon: Component<{ class?: string }>;
		/** Shown when the VM is in this status — the button is disabled otherwise. */
		applicable: import('./list.svelte').VmStatus[];
		variant: 'primary' | 'neutral' | 'danger';
		/** Toast message key fired on a successful action. */
		successToast: (name: string) => string;
	};

	const ACTIONS: readonly ActionDef[] = [
		{ kind: 'start', label: () => m['vms.action.start'](), icon: PlayIcon, applicable: ['stopped'], variant: 'primary', successToast: (name) => m['toast.vmStarted']({ name }) },
		{ kind: 'shutdown', label: () => m['vms.action.shutdown'](), icon: PowerOffIcon, applicable: ['running'], variant: 'neutral', successToast: (name) => m['toast.vmShutdown']({ name }) },
		{ kind: 'stop', label: () => m['vms.action.stop'](), icon: StopIcon, applicable: ['running'], variant: 'danger', successToast: (name) => m['toast.vmStopped']({ name }) },
		{ kind: 'reboot', label: () => m['vms.action.reboot'](), icon: RestartIcon, applicable: ['running'], variant: 'neutral', successToast: (name) => m['toast.vmRebooted']({ name }) },
		{ kind: 'reset', label: () => m['vms.action.reset'](), icon: ResetIcon, applicable: ['running', 'paused'], variant: 'danger', successToast: (name) => m['toast.vmReset']({ name }) },
		{ kind: 'pause', label: () => m['vms.action.pause'](), icon: PauseIcon, applicable: ['running'], variant: 'neutral', successToast: (name) => m['toast.vmPaused']({ name }) },
		{ kind: 'resume', label: () => m['vms.action.resume'](), icon: PlayIcon, applicable: ['paused'], variant: 'primary', successToast: (name) => m['toast.vmResumed']({ name }) }
	] as const;

	interface Props {
		onDelete: () => void;
	}

	let { onDelete }: Props = $props();

	function isApplicable(action: ActionDef): boolean {
		return store.entity !== null && action.applicable.includes(store.entity.status);
	}

	// Seven actions sit in this bar and at most two apply at a time, so only
	// the state-appropriate one is filled. The forceful actions (stop, reset)
	// stay bordered and destructive-tinted rather than solid red: seven solid
	// buttons, five of them greyed out, read as noise instead of as a choice.
	// Only the applicable action carries its own weight; the rest fall back to
	// the neutral bordered shape so a disabled orange or red fill never sits
	// in the bar pretending to be a live control.
	const BUTTON_VARIANT: Record<ActionDef['variant'], 'primary' | 'secondary' | 'outline'> = {
		primary: 'primary',
		danger: 'outline',
		neutral: 'secondary'
	};

	const DANGER_TINT =
		'border-destructive/40 text-destructive hover:border-destructive/60 hover:bg-destructive/10 hover:text-destructive';

	async function handleAction(kind: VmAction): Promise<void> {
		const actionDef = ACTIONS.find((a) => a.kind === kind);
		const vmName = store.entity?.name ?? '';
		const hadErrorBefore = store.actionError;
		await store.action(kind);
		// store.action sets actionError on failure and clears it on success.
		if (store.actionError && store.actionError !== hadErrorBefore) {
			toast.error(m['toast.vmActionFailed']({ error: store.actionError }));
		} else if (actionDef && !store.actionError) {
			toast.success(actionDef.successToast(vmName));
		}
	}
</script>

<div class="flex flex-wrap items-center gap-2" data-testid="vm-action-bar">
	{#each ACTIONS as action (action.kind)}
		{@const applicable = isApplicable(action)}
		<Button
			size="sm"
			variant={applicable ? BUTTON_VARIANT[action.variant] : 'secondary'}
			class={applicable && action.variant === 'danger' ? DANGER_TINT : ''}
			disabled={store.actionInFlight || !applicable}
			onclick={() => handleAction(action.kind)}
			data-testid="vm-action-{action.kind}"
			title={action.label()}
			label={action.label()}
		>
			<action.icon class="h-4 w-4" />
			{action.label()}
		</Button>
	{/each}

	<Button
		size="sm"
		variant="outline"
		class="ml-auto {DANGER_TINT}"
		disabled={store.deleteInFlight}
		onclick={onDelete}
		data-testid="vm-action-delete"
		title={m['vms.action.delete']()}
		label={m['vms.action.delete']()}
	>
		<TrashIcon class="h-4 w-4" />
		{m['vms.action.delete']()}
	</Button>
</div>

{#if store.actionError}
	<p role="alert" class="mt-2 text-sm text-destructive" data-testid="vm-action-error">
		{store.actionError}
	</p>
{/if}

{#if store.actionInFlight && store.entity}
	<p role="status" aria-live="polite" class="sr-only" data-testid="vm-action-aria">
		{m['vms.detail.statusChanged']({ name: store.entity.name, status: store.entity.status })}
	</p>
{/if}
