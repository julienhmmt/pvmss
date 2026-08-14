<script lang="ts">
	import { getVmBulkContext } from './bulk.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const bulk = getVmBulkContext();

	const ACTIONS: readonly { value: string; label: () => string }[] = [
		{ value: 'start', label: () => m['vms.action.start']() },
		{ value: 'stop', label: () => m['vms.action.stop']() },
		{ value: 'shutdown', label: () => m['vms.action.shutdown']() },
		{ value: 'reboot', label: () => m['vms.action.reboot']() },
		{ value: 'reset', label: () => m['vms.action.reset']() }
	] as const;

	let selectedAction = $state<string>('start');
	let submitError = $state<string | null>(null);

	const summary = $derived(bulk.resultSummary);

	async function handleSubmit(event: Event): Promise<void> {
		event.preventDefault();
		if (!bulk.hasSelection || bulk.submitting) return;
		submitError = null;
		try {
			await bulk.submitBulkAction(selectedAction);
		} catch (err) {
			submitError = err instanceof Error ? err.message : m['vms.bulk.errorDefault']();
		}
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
		class="sticky top-0 z-10 mb-4 flex flex-wrap items-center gap-3 rounded-md border border-border bg-background/95 p-3 backdrop-blur"
		data-testid="vm-bulk-action-bar"
	>
		<span class="text-sm font-medium" data-testid="vm-bulk-selected-count">
			{m['common.selectedCount']({ count: bulk.selectedCount })}
		</span>

		<form class="flex items-center gap-2" onsubmit={handleSubmit}>
			<label for="vm-bulk-action" class="sr-only">{m['common.bulkAction']()}</label>
			<select
				id="vm-bulk-action"
				class="rounded-md border border-border bg-background px-3 py-1.5 text-sm"
				bind:value={selectedAction}
				data-testid="vm-bulk-action-select"
			>
				{#each ACTIONS as action (action.value)}
					<option value={action.value}>{action.label()}</option>
				{/each}
			</select>

			<button
				type="submit"
				class="rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
				disabled={bulk.submitting}
				data-testid="vm-bulk-action-submit"
			>
				{bulk.submitting ? m['common.applying']() : m['common.apply']()}
			</button>
		</form>

		<button
			type="button"
			class="rounded-md border border-border bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted"
			onclick={handleClearSelection}
			data-testid="vm-bulk-clear-selection"
		>
			{m['common.clearSelection']()}
		</button>

		{#if submitError}
			<p role="alert" class="text-sm text-destructive" data-testid="vm-bulk-submit-error">
				{submitError}
			</p>
		{/if}

		{#if bulk.lastResult}
			<div class="flex items-center gap-3" data-testid="vm-bulk-result-summary">
				<span class="text-sm">
					<span class="font-medium text-success">{summary.ok}</span> {m['common.succeeded']()}
					·
					<span class="font-medium text-destructive">{summary.error}</span> {m['common.failed']()}
				</span>
				<button
					type="button"
					class="text-sm text-muted-foreground hover:underline"
					onclick={handleDismissResult}
					data-testid="vm-bulk-dismiss-result"
				>
					{m['common.dismiss']()}
				</button>
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
