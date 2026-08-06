<script lang="ts">
	import {
		getVmListContext,
		SORTABLE_COLUMNS,
		type VmSortBy,
		type VmStatus
	} from './list.svelte';

	const store = getVmListContext();

	const COLUMN_LABELS: Record<VmSortBy, string> = {
		vmid: 'ID',
		name: 'Name',
		node: 'Node',
		status: 'Status',
		cpu: 'CPU',
		memory: 'Memory'
	};

	const STATUS_OPTIONS: readonly { value: VmStatus | ''; label: string }[] = [
		{ value: '', label: 'All statuses' },
		{ value: 'running', label: 'Running' },
		{ value: 'stopped', label: 'Stopped' },
		{ value: 'paused', label: 'Paused' }
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

	let pageCount = $derived(
		store.result === null ? 1 : Math.max(1, Math.ceil(store.result.total / store.result.pageSize))
	);
</script>

<div class="mb-4 flex flex-wrap items-center gap-3">
	<div class="flex-1">
		<label for="vm-search" class="sr-only">Search VMs by name, tag, or ID</label>
		<input
			id="vm-search"
			type="search"
			placeholder="Search by name, tag, or ID…"
			class="w-full max-w-sm rounded-md border border-border bg-background px-3 py-1.5 text-sm"
			value={store.search}
			oninput={(event) => store.applySearch(event.currentTarget.value)}
			data-testid="vm-search"
		/>
	</div>

	<label class="sr-only" for="vm-status-filter">Filter by status</label>
	<select
		id="vm-status-filter"
		class="rounded-md border border-border bg-background px-3 py-1.5 text-sm"
		value={store.status}
		onchange={handleStatusChange}
		data-testid="vm-status-filter"
	>
		{#each STATUS_OPTIONS as option (option.value)}
			<option value={option.value}>{option.label}</option>
		{/each}
	</select>

	<label class="sr-only" for="vm-node-filter">Filter by node</label>
	<select
		id="vm-node-filter"
		class="rounded-md border border-border bg-background px-3 py-1.5 text-sm"
		value={store.node}
		onchange={handleNodeChange}
		data-testid="vm-node-filter"
	>
		<option value="">All nodes</option>
		{#each store.result?.availableNodes ?? [] as node (node)}
			<option value={node}>{node}</option>
		{/each}
	</select>

	{#if store.result?.quota}
		<p class="text-sm text-muted-foreground" data-testid="vm-quota">
			{#if store.result.quota.allowed === -1}
				{store.result.quota.used} VMs · unlimited quota
			{:else}
				{store.result.quota.used} / {store.result.quota.allowed} VMs
			{/if}
		</p>
	{/if}
</div>

{#if store.error}
	<p role="alert" class="mb-4 text-sm text-destructive" data-testid="vm-list-error">{store.error}</p>
{/if}

{#if store.result && store.result.items.length === 0}
	{#if store.result.emptyReason === 'no_vms_owned'}
		<p class="py-8 text-center text-muted-foreground" data-testid="vm-empty-owned">
			You don't have any VMs yet.
		</p>
	{:else}
		<p class="py-8 text-center text-muted-foreground" data-testid="vm-empty-match">
			No VMs match your search or filters.
		</p>
	{/if}
{:else}
	<table class="w-full border-collapse text-left text-sm">
		<caption class="sr-only">Virtual machines</caption>
		<thead>
			<tr class="border-b border-border">
				{#each SORTABLE_COLUMNS as column (column)}
					<th scope="col" class="px-3 py-2 font-medium" aria-sort={ariaSort(column)}>
						<button
							type="button"
							class="inline-flex items-center font-medium hover:text-foreground"
							onclick={() => handleSort(column)}
							data-testid="sort-{column}"
						>
							{COLUMN_LABELS[column]}{sortIndicator(column)}
						</button>
					</th>
				{/each}
				{#if store.scope === 'all'}
					<th scope="col" class="px-3 py-2 font-medium">Pool</th>
				{/if}
			</tr>
		</thead>
		<tbody>
			{#each store.result?.items ?? [] as machine (machine.vmid)}
				<tr class="border-b border-border last:border-0" data-testid="vm-row">
					<td class="px-3 py-2 text-muted-foreground">{machine.vmid}</td>
					<td class="px-3 py-2 font-medium">{machine.name}</td>
					<td class="px-3 py-2 text-muted-foreground">{machine.node}</td>
					<td class="px-3 py-2">
						<span
							class="inline-flex items-center rounded-full px-2 py-0.5 text-xs {statusClasses[
								machine.status
							]}"
						>
							{machine.status}
						</span>
					</td>
					<td class="px-3 py-2 text-muted-foreground">{machine.cpuCores} cores</td>
					<td class="px-3 py-2 text-muted-foreground">{formatBytes(machine.memoryTotal)}</td>
					{#if store.scope === 'all'}
						<td class="px-3 py-2 text-muted-foreground">{machine.pool}</td>
					{/if}
				</tr>
			{/each}
		</tbody>
	</table>

	<nav class="mt-4 flex flex-wrap items-center justify-between gap-3" aria-label="VM list pagination">
		<label class="flex items-center gap-2 text-sm text-muted-foreground">
			Rows per page
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

		<div class="flex items-center gap-2">
			<button
				type="button"
				class="rounded-md border border-border bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
				disabled={store.result === null || store.result.page <= 1}
				onclick={() => store.setPage(store.page - 1)}
				data-testid="vm-page-prev"
			>
				Previous
			</button>
			<span class="text-sm text-muted-foreground" data-testid="vm-page-indicator">
				Page {store.result?.page ?? store.page} of {pageCount}
			</span>
			<button
				type="button"
				class="rounded-md border border-border bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
				disabled={store.result === null || store.result.page >= pageCount}
				onclick={() => store.setPage(store.page + 1)}
				data-testid="vm-page-next"
			>
				Next
			</button>
		</div>
	</nav>
{/if}
