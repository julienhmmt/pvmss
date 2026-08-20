<script lang="ts">
	import {
		getVmListContext,
		SORTABLE_COLUMNS,
		type VmSortBy,
		type VmStatus
	} from './list.svelte';
	import { getVmBulkContext } from './bulk.svelte';
	import { resolve } from '$app/paths';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import ChevronDownIcon from '$lib/shared/ui/icons/ChevronDownIcon.svelte';

	const store = getVmListContext();
	const bulk = getVmBulkContext();
	const session = getSessionContext();

	const COLUMN_LABELS: Record<VmSortBy, () => string> = {
		vmid: () => m['vms.list.columnId'](),
		name: () => m['vms.list.columnName'](),
		node: () => m['vms.list.columnNode'](),
		status: () => m['vms.list.columnStatus'](),
		cpu: () => m['vms.list.columnCpu'](),
		memory: () => m['vms.list.columnMemory']()
	};

	const STATUS_OPTIONS: readonly { value: VmStatus | ''; label: () => string }[] = [
		{ value: '', label: () => m['common.allStatuses']() },
		{ value: 'running', label: () => m['common.statusRunning']() },
		{ value: 'stopped', label: () => m['common.statusStopped']() },
		{ value: 'paused', label: () => m['common.statusPaused']() }
	] as const;

	const PAGE_SIZE_OPTIONS: readonly number[] = [10, 25, 50] as const;

	const statusClasses: Record<VmStatus, string> = {
		running: 'bg-success-soft text-success-soft-foreground',
		stopped: 'bg-muted text-muted-foreground',
		paused: 'bg-destructive-soft text-destructive-soft-foreground'
	};

	function formatBytes(bytes: number): string {
		const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
		let value = bytes;
		let unitIndex = 0;
		while (value >= 1024 && unitIndex < units.length - 1) {
			value /= 1024;
			unitIndex += 1;
		}
		return `${value.toFixed(1)} ${units[unitIndex]}`;
	}

	function ariaSort(column: VmSortBy): 'ascending' | 'descending' | 'none' {
		if (store.sortBy !== column) return 'none';
		return store.sortDir === 'asc' ? 'ascending' : 'descending';
	}

	function sortIndicator(column: VmSortBy): string {
		if (store.sortBy !== column) return '';
		return store.sortDir === 'asc' ? ' ↑' : ' ↓';
	}

	function sortIndicatorClass(column: VmSortBy): string {
		return store.sortBy === column ? 'text-primary' : 'text-muted-foreground-subtle';
	}

	function handleSort(column: VmSortBy): void {
		store.setSort(column);
	}

	function handleStatusChange(event: Event): void {
		store.setStatus((event.currentTarget as HTMLSelectElement).value as VmStatus | '');
	}

	function handleNodeChange(event: Event): void {
		store.setNode((event.currentTarget as HTMLSelectElement).value);
	}

	function handlePageSizeChange(event: Event): void {
		store.setPageSize(Number((event.currentTarget as HTMLSelectElement).value));
	}

	function handleRowToggle(cluster: string, vmid: number): void {
		bulk.toggle({ cluster, vmid });
	}

	function handleSelectAllOnPage(event: Event): void {
		const checked = (event.currentTarget as HTMLInputElement).checked;
		const items = store.result?.items ?? [];
		if (checked) {
			bulk.selectPage(items);
		} else {
			bulk.clearPage(items);
		}
	}

	let pageCount = $derived(
		store.result === null ? 1 : Math.max(1, Math.ceil(store.result.total / store.result.pageSize))
	);

	let pageItems = $derived(store.result?.items ?? []);
	let allOnPageSelected = $derived(bulk.pageAllSelected(pageItems));
</script>

<div class="mb-4 flex flex-wrap items-center gap-3">
	<div class="flex-1">
		<label for="vm-search" class="sr-only">{m['common.search']()}</label>
		<input
			id="vm-search"
			type="search"
			placeholder={m['vms.list.searchPlaceholder']()}
			class="w-full max-w-sm rounded-md border border-border bg-background px-3 py-1.5 text-sm"
			value={store.search}
			oninput={(event) => store.applySearch(event.currentTarget.value)}
			data-testid="vm-search"
		/>
	</div>

	<label class="sr-only" for="vm-status-filter">{m['vms.list.filterStatusLabel']()}</label>
	<select
		id="vm-status-filter"
		class="rounded-md border border-border bg-background px-3 py-1.5 text-sm"
		value={store.status}
		onchange={handleStatusChange}
		data-testid="vm-status-filter"
	>
		{#each STATUS_OPTIONS as option (option.value)}
			<option value={option.value}>{option.label()}</option>
		{/each}
	</select>

	<label class="sr-only" for="vm-node-filter">{m['vms.list.filterNodeLabel']()}</label>
	<select
		id="vm-node-filter"
		class="rounded-md border border-border bg-background px-3 py-1.5 text-sm"
		value={store.node}
		onchange={handleNodeChange}
		data-testid="vm-node-filter"
	>
		<option value="">{m['common.allNodes']()}</option>
		{#each store.result?.availableNodes ?? [] as node (node)}
			<option value={node}>{node}</option>
		{/each}
	</select>

	{#if store.result?.quota}
		<p class="text-sm text-muted-foreground" data-testid="vm-quota">
			{#if store.result.quota.allowed === -1}
				{m['vms.list.quotaUnlimited']({ used: store.result.quota.used })}
			{:else}
				{m['vms.list.quotaLimited']({ used: store.result.quota.used, allowed: store.result.quota.allowed })}
			{/if}
		</p>
	{/if}
</div>

{#if store.error}
	<p role="alert" class="mb-4 text-sm text-destructive" data-testid="vm-list-error">{store.error}</p>
{/if}

{#if store.result && store.result.items.length === 0}
	{#if store.result.emptyReason === 'no_vms_owned'}
		<EmptyState title={m['vms.list.emptyOwned']()} dataTestid="vm-empty-owned">
			{#snippet actions()}
				{#if !session.isAdmin}
					<a
						href={resolve('/vms/create')}
						class="rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground"
					>
						{m['vms.list.create']()}
					</a>
				{/if}
			{/snippet}
		</EmptyState>
	{:else}
		<EmptyState title={m['vms.list.emptyMatch']()} dataTestid="vm-empty-match" />
	{/if}
{:else}
	<table class="pv-responsive-table text-sm">
		<caption class="sr-only">{m['vms.list.caption']()}</caption>
		<thead>
			<tr class="border-b border-border">
				<th scope="col" class="px-3 py-2">
					<input
						type="checkbox"
						class="h-4 w-4 rounded border-border"
						checked={allOnPageSelected}
						onchange={handleSelectAllOnPage}
						data-testid="vm-bulk-select-all"
						aria-label={m['vms.list.selectAll']()}
					/>
				</th>
				<th scope="col" class="px-3 py-2 font-medium">{m['vms.list.columnCluster']()}</th>
				{#each SORTABLE_COLUMNS as column (column)}
					<th scope="col" class="px-3 py-2 font-medium" aria-sort={ariaSort(column)}>
						<button
							type="button"
							class="inline-flex items-center font-medium hover:text-foreground"
							onclick={() => handleSort(column)}
							data-testid="sort-{column}"
						>
							{COLUMN_LABELS[column]()}
							<span class="ml-0.5 {sortIndicatorClass(column)}" aria-hidden="true">{sortIndicator(column)}</span>
						</button>
					</th>
				{/each}
				{#if store.scope === 'all'}
					<th scope="col" class="px-3 py-2 font-medium">{m['vms.list.columnPool']()}</th>
				{/if}
			</tr>
		</thead>
		<tbody>
			{#each store.result?.items ?? [] as machine (`${machine.cluster}:${machine.vmid}`)}
				<tr class="border-b border-border last:border-0" data-testid="vm-row">
					<td class="px-3 py-2" data-nolabel="true">
						<input
							type="checkbox"
							class="h-4 w-4 rounded border-border"
							checked={bulk.isSelected(machine.cluster, machine.vmid)}
							onchange={() => handleRowToggle(machine.cluster, machine.vmid)}
							data-testid="vm-bulk-select-row"
							aria-label={m['vms.list.selectRow']({ name: machine.name })}
						/>
					</td>
					<td class="px-3 py-2 font-mono text-muted-foreground" data-label={m['vms.list.columnCluster']()}>{machine.clusterDisplayName}</td>
					<td class="px-3 py-2 font-mono text-muted-foreground" data-label={m['vms.list.columnId']()}>{machine.vmid}</td>
					<td class="px-3 py-2 font-medium" data-label={m['vms.list.columnName']()}>
						<a
							href={resolve(`/vms/${encodeURIComponent(machine.cluster)}/${machine.vmid}`)}
							class="hover:underline"
							data-testid="vm-row-link"
						>
							{machine.name}
						</a>
					</td>
					<td class="px-3 py-2 font-mono text-muted-foreground" data-label={m['vms.list.columnNode']()}>{machine.node}</td>
					<td class="px-3 py-2" data-label={m['vms.list.columnStatus']()}>
						<span
							class="inline-flex items-center rounded-full px-2 py-0.5 text-xs {statusClasses[
								machine.status
							]}"
						>
							{machine.status}
						</span>
					</td>
					<td class="px-3 py-2 font-mono text-muted-foreground" data-label={m['vms.list.columnCpu']()}>{machine.cpuCores} {m['common.cores']()}</td>
					<td class="px-3 py-2 font-mono text-muted-foreground" data-label={m['vms.list.columnMemory']()}>{formatBytes(machine.memoryTotal)}</td>
					{#if store.scope === 'all'}
						<td class="px-3 py-2 text-muted-foreground" data-label={m['vms.list.columnPool']()}>{machine.pool}</td>
					{/if}
				</tr>
			{/each}
		</tbody>
	</table>

	<nav
		class="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-border pt-3"
		aria-label={m['vms.list.paginationLabel']()}
	>
		<label class="flex items-center gap-2 text-xs text-muted-foreground">
			{m['common.rowsPerPage']()}
			<select
				class="rounded-md border border-border bg-background px-2 py-1 text-sm"
				value={store.pageSize}
				onchange={handlePageSizeChange}
				data-testid="vm-page-size"
			>
				{#each PAGE_SIZE_OPTIONS as size (size)}
					<option value={size}>{size}</option>
				{/each}
			</select>
		</label>

		<div class="flex items-center gap-1 rounded-md border border-border p-1">
			<button
				type="button"
				class="flex h-7 w-7 items-center justify-center rounded hover:bg-muted disabled:cursor-not-allowed disabled:opacity-40"
				disabled={store.result === null || store.result.page <= 1}
				onclick={() => store.setPage(store.page - 1)}
				data-testid="vm-page-prev"
			>
				<ChevronDownIcon class="h-4 w-4 rotate-90" />
				<span class="sr-only">{m['common.previous']()}</span>
			</button>
			<span class="px-2 text-sm text-muted-foreground" data-testid="vm-page-indicator">
				{m['common.pageIndicator']({ current: store.result?.page ?? store.page, total: pageCount })}
			</span>
			<button
				type="button"
				class="flex h-7 w-7 items-center justify-center rounded hover:bg-muted disabled:cursor-not-allowed disabled:opacity-40"
				disabled={store.result === null || store.result.page >= pageCount}
				onclick={() => store.setPage(store.page + 1)}
				data-testid="vm-page-next"
			>
				<ChevronDownIcon class="h-4 w-4 -rotate-90" />
				<span class="sr-only">{m['common.next']()}</span>
			</button>
		</div>
	</nav>
{/if}
