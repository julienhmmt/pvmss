<script lang="ts">
	import type { AuditFilter } from './auditLog.svelte';
	import { getAuditLogContext } from './auditLog.svelte';
	import Button from '$lib/shared/ui/Button.svelte';

	const store = getAuditLogContext();

	let actionFilter = $state('');
	let actorFilter = $state('');
	let vmidFilter = $state('');
	let fromFilter = $state('');
	let toFilter = $state('');

	function applyFilter(): void {
		const filter: AuditFilter = {};
		if (actionFilter) filter.action = actionFilter;
		if (actorFilter) filter.actor = actorFilter;
		if (vmidFilter) {
			const vmid = Number(vmidFilter);
			if (!isNaN(vmid) && Number.isInteger(vmid) && vmid > 0) {
				filter.vmid = vmid;
			}
		}
		if (fromFilter) {
			const fromDate = new Date(fromFilter);
			if (!isNaN(fromDate.getTime())) {
				filter.from = fromDate.toISOString();
			}
		}
		if (toFilter) {
			// eslint-disable-next-line svelte/prefer-svelte-reactivity -- local parse scratch var, not $state
			const toDate = new Date(toFilter);
			if (!isNaN(toDate.getTime())) {
				toDate.setSeconds(59);
				toDate.setMilliseconds(999);
				filter.to = toDate.toISOString();
			}
		}
		store.setFilter(filter);
	}

	function clearFilter(): void {
		actionFilter = '';
		actorFilter = '';
		vmidFilter = '';
		fromFilter = '';
		toFilter = '';
		store.clearFilter();
	}

	function formatTimestamp(ts: string): string {
		return new Date(ts).toLocaleString();
	}
</script>

<svelte:head>
	<title>Audit Log — PVMSS</title>
</svelte:head>

<section class="space-y-4">
	<h2 class="text-xl font-semibold tracking-tight">Audit Log</h2>

	<form class="flex flex-wrap items-end gap-3" onsubmit={(e) => { e.preventDefault(); applyFilter(); }}>
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-muted-foreground">Action</span>
			<input class="rounded-md border border-border bg-background px-3 py-1.5" type="text" bind:value={actionFilter} placeholder="start, stop, vm_create…" />
		</label>
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-muted-foreground">Actor</span>
			<input class="rounded-md border border-border bg-background px-3 py-1.5" type="text" bind:value={actorFilter} placeholder="username" />
		</label>
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-muted-foreground">VM ID</span>
			<input class="rounded-md border border-border bg-background px-3 py-1.5 w-24" type="number" bind:value={vmidFilter} placeholder="101" />
		</label>
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-muted-foreground">From</span>
			<input class="rounded-md border border-border bg-background px-3 py-1.5" type="datetime-local" bind:value={fromFilter} />
		</label>
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-muted-foreground">To</span>
			<input class="rounded-md border border-border bg-background px-3 py-1.5" type="datetime-local" bind:value={toFilter} />
		</label>
		<Button type="submit">Filter</Button>
		<Button variant="secondary" onclick={clearFilter}>Clear</Button>
	</form>

	{#if store.loading}
		<p role="status" aria-live="polite" class="text-muted-foreground">Loading…</p>
	{:else if store.error}
		<p role="alert" class="text-destructive">{store.error}</p>
	{:else}
		<div role="status" aria-live="polite" class="sr-only">{store.entries.length} entries loaded, {store.total} total</div>

		<div class="overflow-x-auto rounded-md border border-border">
			<table class="w-full text-sm">
				<thead class="bg-muted/50 text-left">
					<tr>
						<th class="px-4 py-2 font-medium">Time</th>
						<th class="px-4 py-2 font-medium">Actor</th>
						<th class="px-4 py-2 font-medium">Cluster</th>
						<th class="px-4 py-2 font-medium">VM</th>
						<th class="px-4 py-2 font-medium">Action</th>
					</tr>
				</thead>
				<tbody>
					{#each store.entries as entry (entry.id)}
						<tr class="border-t border-border">
							<td class="px-4 py-2 text-muted-foreground">{formatTimestamp(entry.timestamp)}</td>
							<td class="px-4 py-2">{entry.actor}</td>
							<td class="px-4 py-2">{entry.cluster}</td>
							<td class="px-4 py-2">{entry.vmid}</td>
							<td class="px-4 py-2 font-mono">{entry.action}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		{#if store.entries.length === 0}
			<p class="text-center text-muted-foreground py-4">No audit entries match the current filter.</p>
		{/if}

		<div class="flex items-center justify-between">
			<p class="text-sm text-muted-foreground">
				Page {store.page} of {Math.max(1, Math.ceil(store.total / store.pageSize))} ({store.total} total)
			</p>
			<div class="flex gap-2">
				<Button variant="secondary" size="sm" disabled={store.page <= 1} onclick={() => void store.prevPage()}>Previous</Button>
				<Button variant="secondary" size="sm" disabled={store.page * store.pageSize >= store.total} onclick={() => void store.nextPage()}>Next</Button>
			</div>
		</div>
	{/if}
</section>
