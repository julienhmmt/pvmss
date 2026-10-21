<script lang="ts">
	import type { AuditFilter } from './auditLog.svelte';
	import { getAuditLogContext } from './auditLog.svelte';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Select from '#lib/shared/ui/Select.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';
	import TableCard from '#lib/shared/ui/TableCard.svelte';
	import TableSkeleton from '#lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '#lib/shared/ui/EmptyState.svelte';
	import Pill from '#lib/shared/ui/Pill.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { activityMessage, auditTarget, isKnownAction, knownActions } from '#lib/features/activity/activity.svelte.js';

	const store = getAuditLogContext();

	let actionFilter = $state('');
	let actorFilter = $state('');
	let vmidFilter = $state('');
	let fromFilter = $state('');
	let toFilter = $state('');
	let severityFilter = $state('');
	let selectedDetail = $state<number | null>(null);

	// The server matches the action code exactly, and the table shows labels,
	// so the filter offers the labels and sends the code.
	const actionOptions = [
		{ value: '', label: m['admin.audit.actionAll']() },
		...knownActions()
			.map((code) => ({ value: code, label: activityMessage(code) }))
			.sort((a, b) => a.label.localeCompare(b.label))
	];

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

	/** Pretty-printed JSON for the expanded detail row, or the raw string. */
	function detailBlock(entry: { detail: string }): string {
		try {
			return JSON.stringify(JSON.parse(entry.detail), null, 2);
		} catch {
			return entry.detail;
		}
	}

	function severityTone(severity: string): 'error' | 'warn' | 'off' {
		if (severity === 'critical') return 'error';
		if (severity === 'warning') return 'warn';
		return 'off';
	}

	function severityLabel(severity: string): string {
		if (severity === 'critical') return m['admin.audit.severityCritical']();
		if (severity === 'warning') return m['admin.audit.severityWarning']();
		if (severity === 'info') return m['admin.audit.severityInfo']();
		return severity;
	}

	function formatTimestamp(ts: string): string {
		return new Date(ts).toLocaleString();
	}
</script>

<section class="space-y-4" aria-label={m['admin.audit.title']()}>
	{#if store.loading}
		<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
		<TableSkeleton columns={5} />
	{:else if store.error}
		<Alert>{store.error}</Alert>
	{:else}
		<div role="status" aria-live="polite" class="sr-only">{m['admin.audit.entriesLoaded']({ loaded: store.entries.length, total: store.total })}</div>

		<TableCard>
			{#snippet toolbar()}
				<form class="flex flex-wrap items-end gap-3" onsubmit={(e) => { e.preventDefault(); applyFilter(); }}>
					<label class="flex flex-col gap-1 text-sm">
						<span class="text-muted-foreground">{m['admin.audit.action']()}</span>
						<Select bind:value={actionFilter} options={actionOptions} />
					</label>
					<label class="flex flex-col gap-1 text-sm">
						<span class="text-muted-foreground">{m['admin.audit.actor']()}</span>
						<TextField bind:value={actorFilter} placeholder={m['admin.audit.actorPlaceholder']()} />
					</label>
					<label class="flex flex-col gap-1 text-sm">
						<span class="text-muted-foreground">{m['admin.audit.vmid']()}</span>
						<TextField class="w-24" type="number" bind:value={vmidFilter} placeholder="101" />
					</label>
					<label class="flex flex-col gap-1 text-sm">
						<span class="text-muted-foreground">{m['admin.audit.from']()}</span>
						<TextField type="datetime-local" bind:value={fromFilter} />
					</label>
					<label class="flex flex-col gap-1 text-sm">
						<span class="text-muted-foreground">{m['admin.audit.to']()}</span>
						<TextField type="datetime-local" bind:value={toFilter} />
					</label>
					<label class="flex flex-col gap-1 text-sm">
						<span class="text-muted-foreground">{m['admin.audit.severity']()}</span>
						<Select
							bind:value={severityFilter}
							options={[
								{ value: '', label: m['admin.audit.severityAll']() },
								{ value: 'critical', label: m['admin.audit.severityCritical']() },
								{ value: 'warning', label: m['admin.audit.severityWarning']() },
								{ value: 'info', label: m['admin.audit.severityInfo']() }
							]}
						/>
					</label>
					<Button type="submit">{m['common.filter']()}</Button>
					<Button variant="secondary" onclick={clearFilter}>{m['admin.audit.clear']()}</Button>
				</form>
			{/snippet}
			<table class="pv-table pv-responsive-table">
				<caption class="sr-only">{m['admin.audit.title']()}</caption>
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
						<tr>
							<td class="text-muted-foreground" data-label={m['admin.audit.time']()}>{formatTimestamp(entry.timestamp)}</td>
							<td data-label={m['admin.audit.actor']()}>{entry.actor}</td>
							<td data-label={m['common.cluster']()}>{entry.cluster}</td>
							<td data-label={m['admin.audit.columnVm']()}>{entry.vmid ?? ''}</td>
							<td data-label={m['admin.audit.action']()}>
								{#if isKnownAction(entry.action)}
									{activityMessage(entry.action)}
								{:else}
									<!-- A missing label stays visible as the raw code, muted so review spots it. -->
									<span class="font-mono text-muted-foreground" data-testid="audit-action-unknown">{entry.action}</span>
								{/if}
							</td>
							<td data-label={m['admin.audit.target']()}>{auditTarget(entry.targetType, entry.targetId)}</td>
							<td data-label={m['admin.audit.severity']()}>
								<Pill tone={severityTone(entry.severity)} label={severityLabel(entry.severity)} />
							</td>
							<td data-label={m['admin.audit.detail']()}>
								{#if summary.trim() === ''}
									<span class="text-muted-foreground">{m['admin.audit.noDetail']()}</span>
								{:else}
									<button
										type="button"
										class="pv-focus rounded-sm text-left hover:underline"
										aria-expanded={selectedDetail === entry.id}
										aria-controls="audit-detail-{entry.id}"
										onclick={() => selectedDetail = selectedDetail === entry.id ? null : entry.id}
									>
										{summary}
									</button>
								{/if}
							</td>
						</tr>
						{#if selectedDetail === entry.id}
							<tr id="audit-detail-{entry.id}">
								<td colspan={8} data-nolabel="true">
									<pre class="max-h-96 overflow-auto whitespace-pre-wrap break-words rounded-md bg-muted p-4 font-mono text-xs">{detailBlock(entry)}</pre>
								</td>
							</tr>
						{/if}
					{:else}
						<tr><td colspan={8} class="p-0">
							<EmptyState title={m['admin.audit.empty']()} />
						</td></tr>
					{/each}
				</tbody>
			</table>
		</TableCard>

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
