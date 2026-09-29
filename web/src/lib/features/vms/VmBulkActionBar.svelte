<script lang="ts">
	import {
		bulkConfirmationKind,
		getVmBulkContext,
		type BulkConfirmationKind
	} from './bulk.svelte';
	import { getVmListContext } from './list.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import ConfirmDialog from '$lib/shared/ui/ConfirmDialog.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const bulk = getVmBulkContext();
	const list = getVmListContext();

	const ACTIONS: readonly { value: string; label: () => string }[] = [
		{ value: 'start', label: () => m['vms.action.start']() },
		{ value: 'stop', label: () => m['vms.action.stop']() },
		{ value: 'shutdown', label: () => m['vms.action.shutdown']() },
		{ value: 'reboot', label: () => m['vms.action.reboot']() },
		{ value: 'reset', label: () => m['vms.action.reset']() }
	] as const;

	/** Copy for each confirmation kind `bulkConfirmationKind` can return. */
	interface Confirmation {
		title: (count: number) => string;
		message: (count: number) => string;
		confirmLabel: () => string;
	}

	const CONFIRMATIONS: Record<BulkConfirmationKind, Confirmation> = {
		forceStop: {
			title: (count) => m['vms.bulk.confirm.forceStop.title']({ count }),
			message: (count) => m['vms.bulk.confirm.forceStop.message']({ count }),
			confirmLabel: () => m['vms.confirm.forceStop.confirm']()
		},
		reset: {
			title: (count) => m['vms.bulk.confirm.reset.title']({ count }),
			message: (count) => m['vms.bulk.confirm.reset.message']({ count }),
			confirmLabel: () => m['vms.confirm.reset.confirm']()
		}
	};

	/** How many machine names the confirmation lists before summarising. */
	const AFFECTED_LIMIT = 6;

	let selectedAction = $state<string>('start');
	let submitError = $state<string | null>(null);
	/** The forceful action awaiting confirmation, if any. */
	let pending = $state<BulkConfirmationKind | null>(null);

	const summary = $derived(bulk.resultSummary);
	const confirmation = $derived(pending === null ? null : CONFIRMATIONS[pending]);

	// Selection is per-page, so a target that has since scrolled onto another
	// page falls back to its cluster:vmid identity.
	const affectedLabel = $derived.by(() => {
		const items = list.result?.items ?? [];
		const names = bulk.selectedTargets.map(
			(target) =>
				items.find((item) => item.cluster === target.cluster && item.vmid === target.vmid)?.name ??
				`${target.cluster}:${target.vmid}`
		);
		const shown = names.slice(0, AFFECTED_LIMIT);
		const label = m['vms.bulk.confirm.affected']({ names: shown.join(', ') });
		const extra = names.length - shown.length;
		return extra > 0 ? `${label} ${m['vms.bulk.confirm.more']({ count: extra })}` : label;
	});

	async function runAction(action: string): Promise<void> {
		submitError = null;
		try {
			await bulk.submitBulkAction(action);
		} catch (err) {
			submitError = err instanceof Error ? err.message : m['vms.bulk.errorDefault']();
		}
	}

	async function handleSubmit(event: Event): Promise<void> {
		event.preventDefault();
		if (!bulk.hasSelection || bulk.submitting) return;
		const kind = bulkConfirmationKind(selectedAction);
		if (kind !== null) {
			pending = kind;
			return;
		}
		await runAction(selectedAction);
	}

	async function confirmPending(): Promise<void> {
		if (pending === null) return;
		await runAction(selectedAction);
		pending = null;
	}

	function handleClearSelection(): void {
		bulk.clear();
		bulk.clearResult();
		submitError = null;
	}

	function handleDismissResult(): void {
		bulk.clearResult();
	}
</script>

{#if bulk.hasSelection}
	<div
		class="sticky top-0 z-10 mb-4 flex flex-wrap items-center gap-3 rounded-xl border border-primary/25 bg-sidebar-accent/70 px-4 py-3 shadow-raised backdrop-blur"
		data-testid="vm-bulk-action-bar"
	>
		<span class="text-sm font-medium" data-testid="vm-bulk-selected-count">
			{m['common.selectedCount']({ count: bulk.selectedCount })}
		</span>

		<form class="flex items-center gap-2" onsubmit={handleSubmit}>
			<label for="vm-bulk-action" class="sr-only">{m['common.bulkAction']()}</label>
			<Select
				id="vm-bulk-action"
				bind:value={selectedAction}
				options={ACTIONS.map((action) => ({ value: action.value, label: action.label() }))}
				class="w-44"
				data-testid="vm-bulk-action-select"
			/>

			<Button
				type="submit"
				loading={bulk.submitting}
				disabled={!bulk.hasSelection}
				size="sm"
				data-testid="vm-bulk-action-submit"
			>
				{bulk.submitting ? m['common.applying']() : m['common.apply']()}
			</Button>
		</form>

		<Button
			type="button"
			variant="secondary"
			size="sm"
			onclick={handleClearSelection}
			data-testid="vm-bulk-clear-selection"
		>
			{m['common.clearSelection']()}
		</Button>

		{#if submitError}
			<Alert data-testid="vm-bulk-submit-error">{submitError}</Alert>
		{/if}

		{#if bulk.lastResult}
			<div class="flex items-center gap-3" data-testid="vm-bulk-result-summary">
				<span class="text-sm">
					<span class="font-medium text-success">{summary.ok}</span> {m['common.succeeded']()}
					·
					<span class="font-medium text-destructive">{summary.error}</span> {m['common.failed']()}
				</span>
				<Button
					type="button"
					variant="ghost"
					size="sm"
					onclick={handleDismissResult}
					data-testid="vm-bulk-dismiss-result"
				>
					{m['common.dismiss']()}
				</Button>
			</div>
		{/if}
	</div>
{/if}

{#if bulk.lastResult}
	<div class="mb-4 rounded-md border border-border" data-testid="vm-bulk-result-panel">
		<h2 class="border-b border-border px-3 py-2 text-sm font-medium">
			{m['common.resultsTitle']({ ok: summary.ok, error: summary.error })}
		</h2>
		<ul class="divide-y divide-border">
			{#each bulk.lastResult.results as result (`${result.cluster}:${result.vmid}`)}
				<li class="flex items-center justify-between px-3 py-2 text-sm" data-testid="vm-bulk-result-row">
					<span class="font-mono text-muted-foreground">
						{result.cluster}:{result.vmid}
					</span>
					{#if result.status === 'ok'}
						<span class="font-medium text-success" data-testid="vm-bulk-result-ok">
							{m['common.ok']()}
						</span>
					{:else}
						<span class="font-medium text-destructive" data-testid="vm-bulk-result-error">
							{m['common.error']()} {result.message ?? ''}
						</span>
					{/if}
				</li>
			{/each}
		</ul>
	</div>
{/if}

{#if confirmation}
	<ConfirmDialog
		open={true}
		title={confirmation.title(bulk.selectedCount)}
		message="{confirmation.message(bulk.selectedCount)} {affectedLabel}"
		confirmLabel={confirmation.confirmLabel()}
		cancelLabel={m['common.cancel']()}
		confirming={bulk.submitting}
		testId="vm-bulk-confirm"
		onConfirm={confirmPending}
		onClose={() => (pending = null)}
	/>
{/if}
