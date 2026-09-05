<script lang="ts">
	import { onMount } from 'svelte';
	import { getVmDetailContext } from './detail.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';

	const store = getVmDetailContext();

	const totalPages = $derived(store.auditPageSize > 0 ? Math.max(1, Math.ceil(store.auditTotal / store.auditPageSize)) : 1);

	onMount(() => {
		void store.audit(1);
	});

	function formatTimestamp(value: string): string {
		return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
	}

	function goPrev(): void {
		if (store.auditLoading || store.auditPage <= 1) return;
		void store.audit(store.auditPage - 1);
	}

	function goNext(): void {
		if (store.auditLoading || store.auditPage >= totalPages) return;
		void store.audit(store.auditPage + 1);
	}
</script>

<section class="rounded-xl border border-border bg-card p-6 shadow-card" aria-labelledby="activity-heading" data-testid="vm-activity">
	<h2 id="activity-heading" class="text-lg font-semibold">{m['vm.activity.tab']()}</h2>

	{#if store.auditLoading && (store.auditItems === null || store.auditItems.length === 0)}
		<p role="status" aria-live="polite" class="mt-6 text-sm text-muted-foreground" data-testid="vm-activity-loading">
			{m['common.loading']()}
		</p>
	{:else if store.auditError && store.auditItems === null}
		<Alert data-testid="vm-activity-error" class="mt-6">{store.auditError}</Alert>
	{:else if store.auditItems !== null && store.auditItems.length === 0}
		<EmptyState
			title={m['vm.activity.empty']()}
			dataTestid="vm-activity-empty"
			class="mt-6 rounded-xl border border-dashed border-border"
		/>
	{:else if store.auditItems !== null && store.auditItems.length > 0}
		<div class="mt-6 overflow-x-auto">
			<table class="pv-table" data-testid="vm-activity-table">
				<thead>
					<tr class="border-b border-border text-left text-muted-foreground">
						<th scope="col" class="pr-4 font-medium">{m['vm.activity.actor']()}</th>
						<th scope="col" class="pr-4 font-medium">{m['vm.activity.action']()}</th>
						<th scope="col" class="pr-4 font-medium">{m['vm.activity.time']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each store.auditItems as entry (entry.id)}
						<tr data-testid="vm-activity-row">
							<td class="pr-4">{entry.actor}</td>
							<td class="pr-4 font-mono">{entry.action}</td>
							<td class="pr-4 text-muted-foreground">{formatTimestamp(entry.timestamp)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<div class="mt-4 flex items-center justify-between text-sm text-muted-foreground" data-testid="vm-activity-pagination">
			<span>{m['vm.activity.pagetitle']({ page: store.auditPage, total: totalPages })}</span>
			<div class="flex gap-2">
				<button
					type="button"
					class="rounded-lg border border-border bg-background px-3 py-1.5 font-medium hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
					disabled={store.auditLoading || store.auditPage <= 1}
					onclick={goPrev}
					data-testid="vm-activity-prev"
				>
					{m['common.previous']()}
				</button>
				<button
					type="button"
					class="rounded-lg border border-border bg-background px-3 py-1.5 font-medium hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
					disabled={store.auditLoading || store.auditPage >= totalPages}
					onclick={goNext}
					data-testid="vm-activity-next"
				>
					{m['common.next']()}
				</button>
			</div>
		</div>
	{/if}
</section>
