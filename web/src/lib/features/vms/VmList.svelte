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
	import Alert from '$lib/shared/ui/Alert.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { resolve } from '$app/paths';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import { formatBytes } from '$lib/shared/format-bytes';
	import { post } from '$lib/shared/api/client';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import ButtonLink from '$lib/shared/ui/ButtonLink.svelte';
	import Toolbar from '$lib/shared/ui/Toolbar.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import SortButton from '$lib/shared/ui/SortButton.svelte';
	import SearchIcon from '$lib/shared/ui/icons/SearchIcon.svelte';
	import ChevronDownIcon from '$lib/shared/ui/icons/ChevronDownIcon.svelte';
	import PlayIcon from '$lib/shared/ui/icons/PlayIcon.svelte';
	import PowerOffIcon from '$lib/shared/ui/icons/PowerOffIcon.svelte';
	import RestartIcon from '$lib/shared/ui/icons/RestartIcon.svelte';
	import ConsoleIcon from '$lib/shared/ui/icons/ConsoleIcon.svelte';
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

	// Columns whose cells are figures, not prose: they get the `.num` class
	// (tabular mono, right-aligned) so digits line up down the column.
	const NUMERIC_COLUMNS: ReadonlySet<VmSortBy> = new Set<VmSortBy>(['vmid', 'cpu', 'memory']);

	// Columns that drop out in the 640–899px tablet band (sidebar still a
	// drawer, content still narrow) — Node is the only one from the sortable
	// set; Cluster and Pool are handled where they're each rendered, since
	// neither comes from this loop.
	const TABLET_HIDDEN_COLUMNS: ReadonlySet<VmSortBy> = new Set<VmSortBy>(['node']);

	function thClass(column: VmSortBy): string {
		const classes: string[] = [];
		if (NUMERIC_COLUMNS.has(column)) classes.push('num');
		if (TABLET_HIDDEN_COLUMNS.has(column)) classes.push('pv-table-tablet-hide');
		return classes.join(' ');
	}

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

<Card pad="none">
	<Toolbar>
		{#snippet search()}
			<label for="vm-search" class="sr-only">{m['common.search']()}</label>
			<TextField
				id="vm-search"
				type="search"
				placeholder={m['vms.list.searchPlaceholder']()}
				value={store.search}
				oninput={(event: Event) => store.applySearch((event.currentTarget as HTMLInputElement).value)}
				data-testid="vm-search"
			>
				{#snippet leading()}<SearchIcon class="h-4 w-4" />{/snippet}
			</TextField>
		{/snippet}

		{#snippet filters()}
			<label class="sr-only" for="vm-status-filter">{m['vms.list.filterStatusLabel']()}</label>
			<Select
				id="vm-status-filter"
				class="w-auto min-w-[9rem]"
				value={store.status}
				onchange={handleStatusChange}
				options={STATUS_OPTIONS.map((option) => ({ value: option.value, label: option.label() }))}
				data-testid="vm-status-filter"
			/>

			<label class="sr-only" for="vm-node-filter">{m['vms.list.filterNodeLabel']()}</label>
			<Select
				id="vm-node-filter"
				class="w-auto min-w-[9rem]"
				value={store.node}
				onchange={handleNodeChange}
				options={[
					{ value: '', label: m['common.allNodes']() },
					...(store.result?.availableNodes ?? []).map((node) => ({ value: node, label: node }))
				]}
				data-testid="vm-node-filter"
			/>
		{/snippet}

		{#snippet meta()}
			{#if store.result?.quota}
				<span data-testid="vm-quota">
					{#if store.result.quota.allowed === -1}
						{m['vms.list.quotaUnlimited']({ used: store.result.quota.used })}
					{:else}
						{m['vms.list.quotaLimited']({ used: store.result.quota.used, allowed: store.result.quota.allowed })}
					{/if}
				</span>
			{/if}
		{/snippet}
	</Toolbar>

	{#if store.errorCode === 'inventory_not_ready'}
		<EmptyState
			title={m['vms.list.clusterUnreachableTitle']()}
			description={m['vms.list.clusterUnreachableDescription']()}
			tone="error"
			dataTestid="vm-list-cluster-unreachable"
		>
			{#snippet actions()}
				<Button onclick={() => void handleClusterRetry()} data-testid="vm-list-cluster-retry">
					{m['vms.list.clusterUnreachableRetry']()}
				</Button>
			{/snippet}
		</EmptyState>
	{:else if store.error}
		<Alert data-testid="vm-list-error" class="m-4">{store.error}</Alert>
	{/if}

	{#if store.errorCode !== 'inventory_not_ready' && store.result && store.result.items.length === 0}
		{#if store.result.emptyReason === 'no_vms_owned'}
			<EmptyState title={m['vms.list.emptyOwned']()} dataTestid="vm-empty-owned">
				{#snippet actions()}
					{#if !session.isAdmin}
						<ButtonLink href={resolve('/vms/create')}>{m['vms.list.create']()}</ButtonLink>
					{/if}
				{/snippet}
			</EmptyState>
		{:else}
			<EmptyState title={m['vms.list.emptyMatch']()} dataTestid="vm-empty-match" />
		{/if}
	{:else if store.result}
		<div class="max-h-[calc(100svh-20rem)] overflow-auto">
			<table class="pv-table pv-responsive-table">
				<caption class="sr-only">{m['vms.list.caption']()}</caption>
				<thead>
					<tr>
						<th scope="col" class="w-10">
							<input
								type="checkbox"
								class="h-4 w-4 rounded border-border accent-primary"
								checked={allOnPageSelected}
								onchange={handleSelectAllOnPage}
								data-testid="vm-bulk-select-all"
								aria-label={m['vms.list.selectAll']()}
							/>
						</th>
						{#if store.cluster === ''}
							<th scope="col" class="pv-table-tablet-hide">{m['vms.list.columnCluster']()}</th>
						{/if}
						{#each SORTABLE_COLUMNS as column (column)}
							<th scope="col" class={thClass(column)} aria-sort={ariaSort(column)}>
								<SortButton
									label={COLUMN_LABELS[column]()}
									active={store.sortBy === column}
									direction={store.sortDir}
									onclick={() => handleSort(column)}
									data-testid="sort-{column}"
								/>
							</th>
						{/each}
						{#if store.scope === 'all'}
							<th scope="col" class="pv-table-tablet-hide">{m['vms.list.columnPool']()}</th>
						{/if}
						<th scope="col" class="text-right">{m['vms.list.columnActions']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each store.result?.items ?? [] as machine (`${machine.cluster}:${machine.vmid}`)}
						<tr data-testid="vm-row">
							<td data-nolabel="true">
								<input
									type="checkbox"
									class="h-4 w-4 rounded border-border accent-primary"
									checked={bulk.isSelected(machine.cluster, machine.vmid)}
									onchange={() => handleRowToggle(machine.cluster, machine.vmid)}
									data-testid="vm-bulk-select-row"
									aria-label={m['vms.list.selectRow']({ name: machine.name })}
								/>
							</td>
							{#if store.cluster === ''}
								<td
									class="pv-table-tablet-hide text-muted-foreground"
									data-label={m['vms.list.columnCluster']()}
								>
									{machine.clusterDisplayName}
								</td>
							{/if}
							<td data-label={m['vms.list.columnName']()}>
								<a
									href={resolve(`/vms/${encodeURIComponent(machine.cluster)}/${machine.vmid}`)}
									class="font-medium text-foreground underline-offset-2 hover:text-primary hover:underline"
									data-testid="vm-row-link"
								>
									{machine.name}
								</a>
								{#if machine.tags.length > 0}
									<div class="mt-1 flex flex-wrap gap-1">
										{#each machine.tags as tag (tag)}
											<Pill tone="off" dot={false} label={tag} />
										{/each}
									</div>
								{/if}
							</td>
							<td class="num text-muted-foreground" data-label={m['vms.list.columnId']()}>
								{machine.vmid}
							</td>
							<td
								class="pv-table-tablet-hide whitespace-nowrap font-mono text-muted-foreground"
								data-label={m['vms.list.columnNode']()}
							>
								{machine.node}
							</td>
							<td data-label={m['vms.list.columnStatus']()}>
								<Pill
									tone={statusTone[machine.status]}
									label={statusLabels[machine.status]()}
									pending={isRowActionInFlight(machine.cluster, machine.vmid)}
								/>
							</td>
							<td class="num text-muted-foreground" data-label={m['vms.list.columnCpu']()}>
								{machine.cpuCores}<span class="ml-1 font-sans text-xs">{m['common.coreCount']({ count: machine.cpuCores })}</span>
							</td>
							<td class="num text-muted-foreground" data-label={m['vms.list.columnMemory']()}>
								{formatBytes(machine.memoryTotal)}
							</td>
							{#if store.scope === 'all'}
								<td class="pv-table-tablet-hide text-muted-foreground" data-label={m['vms.list.columnPool']()}>
									{machine.pool}
								</td>
							{/if}
							<td data-label={m['vms.list.columnActions']()} data-nolabel="true">
								<div class="flex items-center justify-end gap-1">
									{#if isRowActionInFlight(machine.cluster, machine.vmid)}
										<span class="flex h-8 w-8 items-center justify-center text-muted-foreground" aria-live="polite">
											<SpinnerIcon class="h-4 w-4" />
										</span>
									{:else}
										{#each QUICK_ACTIONS as action (action.kind)}
											{@const applicable = isQuickActionApplicable(action, machine.status)}
											<Button
												variant="ghost"
												size="icon-sm"
												disabled={!applicable}
												onclick={() => void handleQuickAction(machine.cluster, machine.vmid, machine.name, machine.status, action)}
												data-testid="vm-quick-action-{action.kind}"
												title={action.label()}
												label={action.label()}
												aria-disabled={!applicable}
											>
												{#if action.kind === 'start'}
													<PlayIcon class="h-4 w-4" />
												{:else if action.kind === 'shutdown'}
													<PowerOffIcon class="h-4 w-4" />
												{:else if action.kind === 'reboot'}
													<RestartIcon class="h-4 w-4" />
												{/if}
											</Button>
										{/each}
										{#if machine.status === 'running'}
											<ButtonLink
												variant="ghost"
												size="icon-sm"
												href={resolve('/vms/[cluster]/[vmid]/console', {
													cluster: machine.cluster,
													vmid: String(machine.vmid)
												})}
												target="_blank"
												rel="noopener noreferrer"
												title={m['vms.console.open']()}
												label={m['vms.console.open']()}
												data-testid="vm-row-console"
											>
												<ConsoleIcon class="h-4 w-4" />
											</ButtonLink>
										{/if}
									{/if}
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<nav
			class="flex flex-wrap items-center justify-between gap-3 border-t border-border px-4 py-3"
			aria-label={m['vms.list.paginationLabel']()}
		>
			<label class="flex items-center gap-2 text-xs text-muted-foreground">
				{m['common.rowsPerPage']()}
				<Select
					class="w-auto"
					value={String(store.pageSize)}
					onchange={handlePageSizeChange}
					options={PAGE_SIZE_OPTIONS.map((size) => ({ value: String(size), label: String(size) }))}
					data-testid="vm-page-size"
				/>
			</label>

			<div class="flex items-center gap-2">
				<span class="font-mono text-xs tabular-nums text-muted-foreground" data-testid="vm-page-indicator">
					{m['common.pageIndicator']({ current: store.result?.page ?? store.page, total: pageCount })}
				</span>
				<div class="flex items-center gap-1">
					<Button
						variant="secondary"
						size="icon-sm"
						label={m['common.previous']()}
						disabled={store.result === null || store.result.page <= 1}
						onclick={() => store.setPage(store.page - 1)}
						data-testid="vm-page-prev"
					>
						<ChevronDownIcon class="h-4 w-4 rotate-90" />
					</Button>
					<Button
						variant="secondary"
						size="icon-sm"
						label={m['common.next']()}
						disabled={store.result === null || store.result.page >= pageCount}
						onclick={() => store.setPage(store.page + 1)}
						data-testid="vm-page-next"
					>
						<ChevronDownIcon class="h-4 w-4 -rotate-90" />
					</Button>
				</div>
			</div>
		</nav>
	{/if}
</Card>
