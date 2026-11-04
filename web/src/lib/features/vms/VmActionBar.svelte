<script lang="ts">
	import { getVmDetailContext, type VmAction } from './detail.svelte';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import ConfirmDialog from '#lib/shared/ui/ConfirmDialog.svelte';
	import { getToastContext } from '#lib/shared/ui/toast.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import PlayIcon from '#lib/shared/ui/icons/PlayIcon.svelte';
	import PowerOffIcon from '#lib/shared/ui/icons/PowerOffIcon.svelte';
	import StopIcon from '#lib/shared/ui/icons/StopIcon.svelte';
	import RestartIcon from '#lib/shared/ui/icons/RestartIcon.svelte';
	import ResetIcon from '#lib/shared/ui/icons/ResetIcon.svelte';
	import PauseIcon from '#lib/shared/ui/icons/PauseIcon.svelte';
	import TrashIcon from '#lib/shared/ui/icons/TrashIcon.svelte';
	import type { Component } from 'svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Menu from '#lib/shared/ui/Menu.svelte';
	import CloneVmDialog from './CloneVmDialog.svelte';

	const store = getVmDetailContext();
	const toast = getToastContext();

	type ActionDef = {
		kind: VmAction;
		label: () => string;
		icon: Component<{ class?: string }>;
		/** Shown when the VM is in this status - the button is disabled otherwise. */
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

	/**
	 * The forceful actions cut power to a running guest, so they get a
	 * confirmation step before they fire. `shutdown` and `reboot` ask the
	 * guest to stop itself and are recoverable; `start`, `pause` and `resume`
	 * are not destructive at all.
	 */
	interface Confirmation {
		title: (name: string) => string;
		message: (name: string) => string;
		confirmLabel: () => string;
	}

	const CONFIRMATIONS: Partial<Record<VmAction, Confirmation>> = {
		shutdown: {
			title: () => m['vms.detail.banner.shutdownTitle'](),
			message: () => m['vms.detail.banner.shutdownBody'](),
			confirmLabel: () => m['vms.detail.banner.confirmShutdown']()
		},
		stop: {
			title: (name) => m['vms.confirm.forceStop.title']({ name }),
			message: () => m['vms.confirm.forceStop.message'](),
			confirmLabel: () => m['vms.confirm.forceStop.confirm']()
		},
		reset: {
			title: (name) => m['vms.confirm.reset.title']({ name }),
			message: () => m['vms.confirm.reset.message'](),
			confirmLabel: () => m['vms.confirm.reset.confirm']()
		}
	};

	interface Props {
		onDelete?: () => void;
		hideDelete?: boolean;
		/** Icon-only, and only the actions that apply to the current status (console header). */
		compact?: boolean;
		/** Icon-only secondary actions for the detail header, beside its own start/shutdown button. */
		quick?: boolean;
	}

	let { onDelete = () => {}, hideDelete = false, compact = false, quick = false }: Props = $props();

	/** The forceful action awaiting confirmation, if any. */
	let pending = $state<VmAction | null>(null);
	/** Clone dialog visibility (Duplicate button). */
	let cloneOpen = $state(false);
	const confirmation = $derived(pending === null ? null : (CONFIRMATIONS[pending] ?? null));
	const vmName = $derived(store.entity?.name ?? '');

	function isApplicable(action: ActionDef): boolean {
		return store.entity !== null && action.applicable.includes(store.entity.status);
	}

	// Seven actions sit in this bar and at most two apply at a time. Only the
	// state-appropriate action carries a fill: `primary` (start, resume) in the
	// accent, the forceful `stop` and `reset` in the destructive red so cutting
	// power can never be mistaken for a graceful shutdown. The rest stay on the
	// neutral bordered shape, and every inapplicable action falls back to that
	// same shape so a disabled fill never sits in the bar pretending to be a
	// live control. The two destructive ones also confirm first (CONFIRMATIONS).
	const BUTTON_VARIANT: Record<ActionDef['variant'], 'primary' | 'secondary' | 'destructive'> = {
		primary: 'primary',
		danger: 'destructive',
		neutral: 'secondary'
	};

	async function handleAction(kind: VmAction): Promise<void> {
		const actionDef = ACTIONS.find((a) => a.kind === kind);
		const hadErrorBefore = store.actionError;
		await store.action(kind);
		// store.action sets actionError on failure and clears it on success.
		if (store.actionError && store.actionError !== hadErrorBefore) {
			toast.error(m['toast.vmActionFailed']({ error: store.actionError }));
		} else if (actionDef && !store.actionError) {
			toast.success(actionDef.successToast(vmName));
		}
	}

	function requestAction(kind: VmAction): void {
		if (CONFIRMATIONS[kind]) {
			pending = kind;
			return;
		}
		void handleAction(kind);
	}

	async function confirmPending(): Promise<void> {
		if (pending === null) return;
		await handleAction(pending);
		pending = null;
	}
</script>

{#if quick}
	<!-- Test ids differ from the full bar, which stays mounted in the Configuration tab. -->
	{#each ACTIONS.filter((action) => isApplicable(action) && action.kind !== 'start' && action.kind !== 'shutdown') as action (action.kind)}
		<Button
			size="icon"
			variant={BUTTON_VARIANT[action.variant]}
			disabled={store.actionInFlight}
			onclick={() => requestAction(action.kind)}
			data-testid="vm-quick-{action.kind}"
			title={action.label()}
			label={action.label()}
		>
			<action.icon class="h-4 w-4" />
		</Button>
	{/each}
{:else if compact}
	<!-- Space-constrained bar: the first applicable action keeps a visible
	     icon button, the rest (and Delete) fold into the overflow menu. -->
	{@const applicableActions = ACTIONS.filter(isApplicable)}
	<!-- The visible slot belongs to the state-appropriate primary action;
	     array order only decides when none applies. -->
	{@const lead = applicableActions.find((action) => action.variant === 'primary') ?? applicableActions[0]}
	{@const rest = applicableActions.filter((action) => action !== lead)}
	<div class="flex flex-wrap items-center gap-2" data-testid="vm-action-bar">
		{#if lead}
			<Button
				size="sm"
				variant={BUTTON_VARIANT[lead.variant]}
				disabled={store.actionInFlight}
				onclick={() => requestAction(lead.kind)}
				data-testid="vm-action-{lead.kind}"
				title={lead.label()}
				label={lead.label()}
			>
				<lead.icon class="h-4 w-4" />
			</Button>
		{/if}
		{#if rest.length > 0 || !hideDelete}
			<Menu
				label={m['vms.action.more']()}
				testId="vm-action-menu"
				items={[
					...rest.map((action) => ({
						label: action.label(),
						icon: action.icon,
						destructive: action.variant === 'danger',
						disabled: store.actionInFlight,
						onclick: () => requestAction(action.kind),
						testId: `vm-menu-${action.kind}`
					})),
					...(hideDelete ? [] : [{
						label: m['vms.action.delete'](),
						icon: TrashIcon,
						destructive: true,
						disabled: store.deleteInFlight,
						onclick: onDelete,
						testId: 'vm-menu-delete'
					}])
				]}
			/>
		{/if}
	</div>
{:else}
	<div class="flex flex-wrap items-center gap-2" data-testid="vm-action-bar">
		{#each ACTIONS as action (action.kind)}
			{@const applicable = isApplicable(action)}
			<Button
				size="sm"
				variant={applicable ? BUTTON_VARIANT[action.variant] : 'secondary'}
				disabled={store.actionInFlight || !applicable}
				onclick={() => requestAction(action.kind)}
				data-testid="vm-action-{action.kind}"
				title={action.label()}
				label={action.label()}
			>
				<action.icon class="h-4 w-4" />
				{action.label()}
			</Button>
		{/each}

		{#if store.entity !== null}
			<Button
				size="sm"
				variant="secondary"
				disabled={store.actionInFlight}
				onclick={() => (cloneOpen = true)}
				data-testid="vm-action-clone"
				title={m['vms.clone.button']()}
				label={m['vms.clone.button']()}
			>
				{m['vms.clone.button']()}
			</Button>
		{/if}

		{#if !hideDelete}
			<Button
				size="sm"
				variant="destructive"
				class="ml-auto"
				disabled={store.deleteInFlight}
				onclick={onDelete}
				data-testid="vm-action-delete"
				title={m['vms.action.delete']()}
				label={m['vms.action.delete']()}
			>
				<TrashIcon class="h-4 w-4" />
				{m['vms.action.delete']()}
			</Button>
		{/if}
	</div>
{/if}

{#if confirmation}
	<ConfirmDialog
		open={true}
		title={confirmation.title(vmName)}
		message={confirmation.message(vmName)}
		confirmLabel={confirmation.confirmLabel()}
		cancelLabel={m['common.cancel']()}
		confirming={store.actionInFlight}
		testId="vm-action-confirm"
		onConfirm={confirmPending}
		onClose={() => (pending = null)}
	/>
{/if}

{#if store.entity !== null && !compact && !quick}
	<CloneVmDialog bind:open={cloneOpen} cluster={store.entity.cluster} vmid={store.entity.vmid} sourceName={store.entity.name} />
{/if}

{#if store.actionError && !quick}
	<Alert data-testid="vm-action-error" class="mt-2">{store.actionError}</Alert>
{/if}

{#if store.actionInFlight && store.entity && !quick}
	<p role="status" aria-live="polite" class="sr-only" data-testid="vm-action-aria">
		{m['vms.detail.statusChanged']({ name: store.entity.name, status: store.entity.status })}
	</p>
{/if}
