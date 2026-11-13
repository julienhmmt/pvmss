<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteMap } from 'svelte/reactivity';
	import { getVmDetailContext, type VmAuditEntry } from './detail.svelte';
	import { activityMessage } from '#lib/features/activity/activity.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import { formatDateTime } from '#lib/shared/format-date.js';
	import { ApiRequestError } from '#lib/shared/api/client.js';
	import { activityCsvFilename, buildActivityCsv } from './activity-csv';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import EmptyState from '#lib/shared/ui/EmptyState.svelte';
	import Select from '#lib/shared/ui/Select.svelte';
	import TableSkeleton from '#lib/shared/ui/TableSkeleton.svelte';

	const store = getVmDetailContext();

	const totalPages = $derived(store.auditPageSize > 0 ? Math.max(1, Math.ceil(store.auditTotal / store.auditPageSize)) : 1);

	const DAY_MS = 24 * 60 * 60 * 1000;
	const RANGES = [
		{ value: '24h', ms: DAY_MS },
		{ value: '7d', ms: 7 * DAY_MS },
		{ value: '30d', ms: 30 * DAY_MS },
		{ value: 'all', ms: undefined }
	] as const;
	type RangeValue = (typeof RANGES)[number]['value'];
	const RANGE_LABELS: Record<RangeValue, () => string> = {
		'24h': () => m['vm.activity.range24h'](),
		'7d': () => m['vm.activity.range7d'](),
		'30d': () => m['vm.activity.range30d'](),
		all: () => m['vm.activity.rangeAll']()
	};

	/** Selection survives page changes: entries are kept, not just ids. */
	const selected = new SvelteMap<number, VmAuditEntry>();
	let range = $state<RangeValue>('all');
	let exporting = $state(false);
	let exportError = $state<string | null>(null);

	const rangeOptions = $derived(RANGES.map(({ value }) => ({ value, label: RANGE_LABELS[value]() })));
	const allPageSelected = $derived(
		(store.auditItems?.length ?? 0) > 0 && (store.auditItems ?? []).every((entry) => selected.has(entry.id))
	);
	const somePageSelected = $derived(
		!allPageSelected && (store.auditItems ?? []).some((entry) => selected.has(entry.id))
	);
	const exportLabel = $derived(
		selected.size > 0 ? m['vm.activity.exportSelected']({ count: selected.size }) : m['vm.activity.export']()
	);

	onMount(() => {
		void store.audit(1);
	});

	function formatTimestamp(value: string): string {
		return formatDateTime(value);
	}

	function goPrev(): void {
		if (store.auditLoading || store.auditPage <= 1) return;
		void store.audit(store.auditPage - 1);
	}

	function goNext(): void {
		if (store.auditLoading || store.auditPage >= totalPages) return;
		void store.audit(store.auditPage + 1);
	}

	function togglePage(checked: boolean): void {
		for (const entry of store.auditItems ?? []) {
			if (checked) selected.set(entry.id, entry);
			else selected.delete(entry.id);
		}
	}

	function toggleEntry(entry: VmAuditEntry, checked: boolean): void {
		if (checked) selected.set(entry.id, entry);
		else selected.delete(entry.id);
	}

	function downloadCsv(csv: string, filename: string): void {
		const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }));
		const anchor = document.createElement('a');
		anchor.href = url;
		anchor.download = filename;
		document.body.appendChild(anchor);
		anchor.click();
		document.body.removeChild(anchor);
		URL.revokeObjectURL(url);
	}

	async function exportCsv(): Promise<void> {
		if (exporting) return;
		exporting = true;
		exportError = null;
		try {
			const rangeMs = RANGES.find((preset) => preset.value === range)?.ms;
			const entries =
				selected.size > 0
					? [...selected.values()].sort((a, b) => b.timestamp.localeCompare(a.timestamp))
					: await store.auditAll(rangeMs === undefined ? undefined : Date.now() - rangeMs);
			downloadCsv(buildActivityCsv(entries, activityMessage), activityCsvFilename(store.vmid));
		} catch (err) {
			exportError = err instanceof ApiRequestError ? err.message : m['vm.activity.exportError']();
		} finally {
			exporting = false;
		}
	}
</script>

<Card pad="lg" aria-labelledby="activity-heading" data-testid="vm-activity">
	<h2 id="activity-heading" class="text-lg font-semibold">{m['vm.activity.tab']()}</h2>

	{#if store.auditError && store.auditItems !== null}
		<Alert class="mt-6" data-testid="vm-activity-page-error">{store.auditError}</Alert>
	{/if}
	{#if exportError}
		<Alert class="mt-6" data-testid="vm-activity-export-error">{exportError}</Alert>
	{/if}

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
		<div class="mt-6 flex flex-wrap items-center gap-3" data-testid="vm-activity-export-bar">
			<div class="w-44">
				<Select
					bind:value={range}
					options={rangeOptions}
					disabled={exporting || selected.size > 0}
					aria-label={m['vm.activity.range']()}
					data-testid="vm-activity-range"
				/>
			</div>
			<Button
				variant="secondary"
				size="sm"
				disabled={exporting}
				loading={exporting}
				onclick={() => void exportCsv()}
				data-testid="vm-activity-export"
			>
				{exportLabel}
			</Button>
		</div>

		<div class="mt-4 overflow-x-auto">
			<table class="pv-table pv-responsive-table" data-testid="vm-activity-table">
				<caption class="sr-only">{m['vm.activity.tab']()}</caption>
				<thead>
					<tr>
						<th scope="col" class="w-8">
							<input
								type="checkbox"
								class="h-4 w-4 rounded border-border accent-primary"
								checked={allPageSelected}
								indeterminate={somePageSelected}
								onchange={(event) => togglePage(event.currentTarget.checked)}
								aria-label={m['vm.activity.selectAll']()}
								data-testid="vm-activity-select-all"
							/>
						</th>
						<th scope="col" class="font-medium">{m['vm.activity.actor']()}</th>
						<th scope="col" class="font-medium">{m['vm.activity.action']()}</th>
						<th scope="col" class="font-medium">{m['vm.activity.time']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each store.auditItems as entry (entry.id)}
						<tr data-testid="vm-activity-row">
							<td>
								<input
									type="checkbox"
									class="h-4 w-4 rounded border-border accent-primary"
									checked={selected.has(entry.id)}
									onchange={(event) => toggleEntry(entry, event.currentTarget.checked)}
									aria-label={m['vm.activity.selectRow']({ actor: entry.actor })}
									data-testid="vm-activity-select-row"
								/>
							</td>
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
