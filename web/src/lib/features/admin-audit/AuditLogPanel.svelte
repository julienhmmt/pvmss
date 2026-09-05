<script lang="ts">
	import type { AuditFilter } from './auditLog.svelte';
	import { getAuditLogContext } from './auditLog.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getAuditLogContext();

	let actionFilter = $state('');
	let actorFilter = $state('');
	let vmidFilter = $state('');
	let fromFilter = $state('');
	let toFilter = $state('');
	let severityFilter = $state('');
	let selectedDetail = $state<number | null>(null);

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
		if (severityFilter) {
			filter.severity = severityFilter;
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
		severityFilter = '';
		store.clearFilter();
	}

	function detailSummary(entry: { detail: string }): string {
		try {
			const parsed = JSON.parse(entry.detail) as { summary?: string };
			return parsed.summary ?? entry.detail;
		} catch {
			return entry.detail;
		}
	}

	function formatTimestamp(ts: string): string {
		return new Date(ts).toLocaleString();
	}
</script>

<svelte:head>
	<title>{m['admin.audit.pageTitle']()}</title>
</svelte:head>

<section class="space-y-4">
	<h2 class="text-xl font-semibold tracking-tight">{m['admin.audit.title']()}</h2>

	<form class="flex flex-wrap items-end gap-3" onsubmit={(e) => { e.preventDefault(); applyFilter(); }}>
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-muted-foreground">{m['admin.audit.action']()}</span>
			<input class="pv-input" type="text" bind:value={actionFilter} placeholder={m['admin.audit.actionPlaceholder']()} />
		</label>
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-muted-foreground">{m['admin.audit.actor']()}</span>
			<input class="pv-input" type="text" bind:value={actorFilter} placeholder={m['admin.audit.actorPlaceholder']()} />
		</label>
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-muted-foreground">{m['admin.audit.vmid']()}</span>
			<input class="pv-input w-24" type="number" bind:value={vmidFilter} placeholder="101" />
		</label>
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-muted-foreground">{m['admin.audit.from']()}</span>
			<input class="pv-input" type="datetime-local" bind:value={fromFilter} />
		</label>
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-muted-foreground">{m['admin.audit.to']()}</span>
			<input class="pv-input" type="datetime-local" bind:value={toFilter} />
		</label>
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-muted-foreground">{m['admin.audit.severity']()}</span>
			<select class="pv-input" bind:value={severityFilter}>
				<option value="">{m['admin.audit.severityAll']()}</option>
				<option value="critical">{m['admin.audit.severityCritical']()}</option>
				<option value="warning">{m['admin.audit.severityWarning']()}</option>
				<option value="info">{m['admin.audit.severityInfo']()}</option>
			</select>
		</label>
		<Button type="submit">{m['common.filter']()}</Button>
		<Button variant="secondary" onclick={clearFilter}>{m['admin.audit.clear']()}</Button>
	</form>

	{#if store.loading}
		<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
		<TableSkeleton columns={5} />
	{:else if store.error}
		<Alert>{store.error}</Alert>
	{:else}
		<div role="status" aria-live="polite" class="sr-only">{m['admin.audit.entriesLoaded']({ loaded: store.entries.length, total: store.total })}</div>

		<div class="overflow-x-auto rounded-md border border-border">
			<table class="pv-table">
				<thead>
					<tr>
						<th class="font-medium">{m['admin.audit.time']()}</th>
						<th class="font-medium">{m['admin.audit.actor']()}</th>
						<th class="font-medium">{m['common.cluster']()}</th>
						<th class="font-medium">{m['admin.audit.columnVm']()}</th>
						<th class="font-medium">{m['admin.audit.action']()}</th>
						<th class="font-medium">{m['admin.audit.target']()}</th>
						<th class="font-medium">{m['admin.audit.severity']()}</th>
						<th class="font-medium">{m['admin.audit.detail']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each store.entries as entry (entry.id)}
						{@const summary = detailSummary(entry)}
						<tr class="border-t border-border">
							<td class="text-muted-foreground">{formatTimestamp(entry.timestamp)}</td>
							<td>{entry.actor}</td>
							<td>{entry.cluster}</td>
							<td>{entry.vmid ?? ''}</td>
							<td class="font-mono">{entry.action}</td>
							<td>{entry.targetType ? `${entry.targetType}:${entry.targetId}` : ''}</td>
							<td>{entry.severity}</td>
							<td>
								<button type="button" class="text-left hover:underline" onclick={() => selectedDetail = selectedDetail === entry.id ? null : entry.id}>
									{summary}
								</button>
								{#if selectedDetail === entry.id}
									<pre class="mt-1 max-w-xs whitespace-pre-wrap break-all rounded bg-muted p-1 text-xs">{entry.detail}</pre>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<EmptyState title={m['admin.audit.empty']()} />

		<div class="flex items-center justify-between">
			<p class="text-sm text-muted-foreground">
				{m['admin.audit.pageInfo']({ page: store.page, totalPages: Math.max(1, Math.ceil(store.total / store.pageSize)), total: store.total })}
			</p>
			<div class="flex gap-2">
				<Button variant="secondary" size="sm" disabled={store.page <= 1} onclick={() => void store.prevPage()}>{m['common.previous']()}</Button>
				<Button variant="secondary" size="sm" disabled={store.page * store.pageSize >= store.total} onclick={() => void store.nextPage()}>{m['common.next']()}</Button>
			</div>
		</div>
	{/if}
</section>
