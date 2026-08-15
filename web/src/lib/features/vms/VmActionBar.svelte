<script lang="ts">
	import { getVmDetailContext, type VmAction } from './detail.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getVmDetailContext();
	const toast = getToastContext();

	type ActionDef = {
		kind: VmAction;
		label: () => string;
		/** Shown when the VM is in this status — the button is disabled otherwise. */
		applicable: import('./list.svelte').VmStatus[];
		variant: 'primary' | 'neutral' | 'danger';
		/** Toast message key fired on a successful action. */
		successToast: (name: string) => string;
	};

	const ACTIONS: readonly ActionDef[] = [
		{ kind: 'start', label: () => m['vms.action.start'](), applicable: ['stopped'], variant: 'primary', successToast: (name) => m['toast.vmStarted']({ name }) },
		{ kind: 'shutdown', label: () => m['vms.action.shutdown'](), applicable: ['running'], variant: 'neutral', successToast: (name) => m['toast.vmShutdown']({ name }) },
		{ kind: 'stop', label: () => m['vms.action.stop'](), applicable: ['running'], variant: 'danger', successToast: (name) => m['toast.vmStopped']({ name }) },
		{ kind: 'reboot', label: () => m['vms.action.reboot'](), applicable: ['running'], variant: 'neutral', successToast: (name) => m['toast.vmRebooted']({ name }) },
		{ kind: 'reset', label: () => m['vms.action.reset'](), applicable: ['running', 'paused'], variant: 'danger', successToast: (name) => m['toast.vmReset']({ name }) }
	] as const;

	interface Props {
		onDelete: () => void;
	}

	let { onDelete }: Props = $props();

	function isApplicable(action: ActionDef): boolean {
		return store.entity !== null && action.applicable.includes(store.entity.status);
	}

	function variantClass(variant: ActionDef['variant']): string {
		switch (variant) {
			case 'primary':
				return 'bg-primary text-primary-foreground hover:bg-primary/90';
			case 'danger':
				return 'bg-destructive text-destructive-foreground hover:bg-destructive/90';
			default:
				return 'border border-border bg-background hover:bg-muted';
		}
	}

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
		<button
			type="button"
			class="rounded-md px-3 py-1.5 text-sm font-medium disabled:cursor-not-allowed disabled:opacity-50 {variantClass(action.variant)}"
			disabled={store.actionInFlight || !isApplicable(action)}
			onclick={() => handleAction(action.kind)}
			data-testid="vm-action-{action.kind}"
		>
			{action.label()}
		</button>
	{/each}

	<button
		type="button"
		class="ml-auto rounded-md border border-destructive/30 bg-destructive/5 px-3 py-1.5 text-sm font-medium text-destructive hover:bg-destructive/10 disabled:cursor-not-allowed disabled:opacity-50"
		disabled={store.deleteInFlight}
		onclick={onDelete}
		data-testid="vm-action-delete"
	>
		{m['vms.action.delete']()}
	</button>
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
