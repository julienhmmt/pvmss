<script lang="ts">
	import { onMount } from 'svelte';
	import { getVmDetailContext } from './detail.svelte';
	import { activityMessage } from '$lib/features/activity/activity.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';

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

<Card pad="lg" aria-labelledby="activity-heading" data-testid="vm-activity">
	<h2 id="activity-heading" class="text-lg font-semibold">{m['vm.activity.tab']()}</h2>

	{#if store.auditLoading && (store.auditItems === null || store.auditItems.length === 0)}
		<div role="status" aria-live="polite" class="mt-6" data-testid="vm-activity-loading">
			<span class="sr-only">{m['common.loading']()}</span>
			<TableSkeleton columns={3} />
		</div>
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
			<table class="pv-table pv-responsive-table" data-testid="vm-activity-table">
				<caption class="sr-only">{m['vm.activity.tab']()}</caption>
				<thead>
					<tr>
						<th scope="col" class="font-medium">{m['vm.activity.actor']()}</th>
						<th scope="col" class="font-medium">{m['vm.activity.action']()}</th>
						<th scope="col" class="font-medium">{m['vm.activity.time']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each store.auditItems as entry (entry.id)}
						<tr data-testid="vm-activity-row">
							<td data-label={m['vm.activity.actor']()}>{entry.actor}</td>
							<td data-label={m['vm.activity.action']()}>
								{activityMessage(entry.action)}
								<span class="ml-1.5 font-mono text-xs text-muted-foreground">({entry.action})</span>
							</td>
							<td class="text-muted-foreground" data-label={m['vm.activity.time']()}>{formatTimestamp(entry.timestamp)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<div class="mt-4 flex items-center justify-between text-sm text-muted-foreground" data-testid="vm-activity-pagination">
			<span>{m['vm.activity.pagetitle']({ page: store.auditPage, total: totalPages })}</span>
			<div class="flex gap-2">
				<Button
					variant="secondary"
					size="sm"
					disabled={store.auditLoading || store.auditPage <= 1}
					onclick={goPrev}
					data-testid="vm-activity-prev"
				>
					{m['common.previous']()}
				</Button>
				<Button
					variant="secondary"
					size="sm"
					disabled={store.auditLoading || store.auditPage >= totalPages}
					onclick={goNext}
					data-testid="vm-activity-next"
				>
					{m['common.next']()}
				</Button>
			</div>
		</div>
	{/if}
</Card>
