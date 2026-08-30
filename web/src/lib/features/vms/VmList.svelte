<script lang="ts">
	import { SvelteMap } from 'svelte/reactivity';
	import {
		getVmListContext,
		SORTABLE_COLUMNS,
		type VmSortBy,
		type VmStatus
	} from './list.svelte';
	import type { VmAction } from './detail.svelte';
	import { getVmBulkContext } from './bulk.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { resolve } from '$app/paths';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import { formatBytes } from '$lib/shared/format-bytes';
	import { post } from '$lib/shared/api/client';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
	import ChevronDownIcon from '$lib/shared/ui/icons/ChevronDownIcon.svelte';
	import PlayIcon from '$lib/shared/ui/icons/PlayIcon.svelte';
	import PowerOffIcon from '$lib/shared/ui/icons/PowerOffIcon.svelte';
	import RestartIcon from '$lib/shared/ui/icons/RestartIcon.svelte';
	import SpinnerIcon from '$lib/shared/ui/icons/SpinnerIcon.svelte';

	const store = getVmListContext();
	const bulk = getVmBulkContext();
	const session = getSessionContext();
	const toast = getToastContext();

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

	const statusTone: Record<VmStatus, 'ok' | 'off' | 'warn'> = {
		running: 'ok',
		stopped: 'off',
		paused: 'warn'
	};

	const statusLabels: Record<VmStatus, () => string> = {
		running: () => m['common.statusRunning'](),
		stopped: () => m['common.statusStopped'](),
		paused: () => m['common.statusPaused']()
	};

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

	async function handleClusterRetry(): Promise<void> {
		try {
			await post('/api/v1/cluster/refresh');
		} catch {
			// The refresh itself may fail (cluster still down) — reload picks
			// up the current error state either way.
		}
		await store.load();
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

	type QuickAction = {
		kind: VmAction;
		label: () => string;
		applicable: VmStatus[];
		successToast: (name: string) => string;
	};

	const QUICK_ACTIONS: readonly QuickAction[] = [
		{
			kind: 'start',
			label: () => m['vms.action.start'](),
			applicable: ['stopped'],
			successToast: (name) => m['toast.vmStarted']({ name })
		},
		{
			kind: 'shutdown',
			label: () => m['vms.action.shutdown'](),
			applicable: ['running'],
			successToast: (name) => m['toast.vmShutdown']({ name })
		},
		{
			kind: 'reboot',
			label: () => m['vms.action.reboot'](),
			applicable: ['running'],
			successToast: (name) => m['toast.vmRebooted']({ name })
		}
	] as const;

	const rowActionInFlight = new SvelteMap<string, VmAction>();

	function rowActionKey(cluster: string, vmid: number): string {
		return `${cluster}:${vmid}`;
	}

	function isRowActionInFlight(cluster: string, vmid: number): boolean {
		return rowActionInFlight.has(rowActionKey(cluster, vmid));
	}

	function isQuickActionApplicable(action: QuickAction, status: VmStatus): boolean {
		return action.applicable.includes(status);
	}

	async function handleQuickAction(
		cluster: string,
		vmid: number,
		name: string,
		status: VmStatus,
		action: QuickAction
	): Promise<void> {
		if (isRowActionInFlight(cluster, vmid) || !isQuickActionApplicable(action, status)) return;
		const key = rowActionKey(cluster, vmid);
		rowActionInFlight.set(key, action.kind);
		try {
			const result = await store.rowAction(cluster, vmid, action.kind);
			if (result.ok) {
				toast.success(action.successToast(name));
			} else {
				toast.error(m['toast.vmActionFailed']({ error: result.error ?? m['error.generic']() }));
			}
		} finally {
			rowActionInFlight.delete(key);
		}
	}

	let pageCount = $derived(
		store.result === null ? 1 : Math.max(1, Math.ceil(store.result.total / store.result.pageSize))
	);

	let pageItems = $derived(store.result?.items ?? []);
	let allOnPageSelected = $derived(bulk.pageAllSelected(pageItems));
</script>

<Card pad="none" class="overflow-hidden">
	<div class="flex flex-wrap items-center gap-3 border-b border-border p-4">
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

	{#if store.errorCode === 'inventory_not_ready'}
		<EmptyState
			title={m['vms.list.clusterUnreachableTitle']()}
			description={m['vms.list.clusterUnreachableDescription']()}
			dataTestid="vm-list-cluster-unreachable"
		>
			{#snippet actions()}
				<button
					type="button"
					class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
					onclick={() => void handleClusterRetry()}
					data-testid="vm-list-cluster-retry"
				>
					{m['vms.list.clusterUnreachableRetry']()}
				</button>
			{/snippet}
		</EmptyState>
	{:else if store.error}
		<p role="alert" class="px-4 py-3 text-sm text-destructive" data-testid="vm-list-error">{store.error}</p>
	{/if}

	{#if store.errorCode !== 'inventory_not_ready' && store.result && store.result.items.length === 0}
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
	{:else if store.result}
		<div class="overflow-x-auto">
			<table class="pv-responsive-table text-sm">
				<caption class="sr-only">{m['vms.list.caption']()}</caption>
				<thead class="bg-muted/40 text-left text-xs uppercase tracking-wide text-muted-foreground">
					<tr class="border-b border-border">
						<th scope="col" class="px-4 py-3">
							<input
								type="checkbox"
								class="h-4 w-4 rounded border-border"
								checked={allOnPageSelected}
								onchange={handleSelectAllOnPage}
								data-testid="vm-bulk-select-all"
								aria-label={m['vms.list.selectAll']()}
							/>
						</th>
						<th scope="col" class="px-4 py-3 font-medium">{m['vms.list.columnCluster']()}</th>
						{#each SORTABLE_COLUMNS as column (column)}
							<th scope="col" class="px-4 py-3 font-medium" aria-sort={ariaSort(column)}>
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
							<th scope="col" class="px-4 py-3 font-medium">{m['vms.list.columnPool']()}</th>
						{/if}
						<th scope="col" class="px-4 py-3 text-right font-medium">{m['vms.list.columnActions']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each store.result?.items ?? [] as machine (`${machine.cluster}:${machine.vmid}`)}
						<tr
							class="border-b border-border last:border-0 transition-colors hover:bg-muted/40"
							data-testid="vm-row"
						>
							<td class="px-4 py-3" data-nolabel="true">
								<input
									type="checkbox"
									class="h-4 w-4 rounded border-border"
									checked={bulk.isSelected(machine.cluster, machine.vmid)}
									onchange={() => handleRowToggle(machine.cluster, machine.vmid)}
									data-testid="vm-bulk-select-row"
									aria-label={m['vms.list.selectRow']({ name: machine.name })}
								/>
							</td>
							<td class="px-4 py-3 font-mono text-muted-foreground" data-label={m['vms.list.columnCluster']()}>
								{machine.clusterDisplayName}
							</td>
							<td class="px-4 py-3" data-label={m['vms.list.columnName']()}>
								<a
									href={resolve(`/vms/${encodeURIComponent(machine.cluster)}/${machine.vmid}`)}
									class="font-medium text-foreground hover:text-primary hover:underline"
									data-testid="vm-row-link"
								>
									{machine.name}
								</a>
								{#if machine.tags.length > 0}
									<div class="mt-1 flex flex-wrap gap-1">
										{#each machine.tags as tag (tag)}
											<span class="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">
												{tag}
											</span>
										{/each}
									</div>
								{/if}
							</td>
							<td class="px-4 py-3 font-mono text-muted-foreground" data-label={m['vms.list.columnId']()}>
								{machine.vmid}
							</td>
							<td class="px-4 py-3 font-mono text-muted-foreground" data-label={m['vms.list.columnNode']()}>
								{machine.node}
							</td>
							<td class="px-4 py-3" data-label={m['vms.list.columnStatus']()}>
								<Pill
									tone={statusTone[machine.status]}
									label={statusLabels[machine.status]()}
									pending={isRowActionInFlight(machine.cluster, machine.vmid)}
							/>
							</td>
							<td class="px-4 py-3 font-mono text-muted-foreground" data-label={m['vms.list.columnCpu']()}>
								{machine.cpuCores} {m['common.cores']()}
							</td>
							<td class="px-4 py-3 font-mono text-muted-foreground" data-label={m['vms.list.columnMemory']()}>
								{formatBytes(machine.memoryTotal)}
							</td>
							{#if store.scope === 'all'}
								<td class="px-4 py-3 text-muted-foreground" data-label={m['vms.list.columnPool']()}>
									{machine.pool}
								</td>
							{/if}
							<td class="px-4 py-3" data-label={m['vms.list.columnActions']()} data-nolabel="true">
								<div class="flex items-center justify-end gap-1">
									{#if isRowActionInFlight(machine.cluster, machine.vmid)}
										<span class="flex h-7 w-7 items-center justify-center text-muted-foreground" aria-live="polite">
											<SpinnerIcon class="h-4 w-4" />
										</span>
									{:else}
										{#each QUICK_ACTIONS as action (action.kind)}
											{@const applicable = isQuickActionApplicable(action, machine.status)}
											<button
												type="button"
												class="flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:bg-transparent disabled:hover:text-muted-foreground"
												disabled={!applicable}
												onclick={() => void handleQuickAction(machine.cluster, machine.vmid, machine.name, machine.status, action)}
												data-testid="vm-quick-action-{action.kind}"
												title={action.label()}
												aria-label={action.label()}
												aria-disabled={!applicable}
											>
												{#if action.kind === 'start'}
													<PlayIcon class="h-4 w-4" />
												{:else if action.kind === 'shutdown'}
													<PowerOffIcon class="h-4 w-4" />
												{:else if action.kind === 'reboot'}
													<RestartIcon class="h-4 w-4" />
												{/if}
											</button>
										{/each}
									{/if}
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<nav
			class="flex flex-wrap items-center justify-between gap-3 border-t border-border p-4"
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
</Card>
