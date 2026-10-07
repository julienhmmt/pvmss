<script lang="ts">
	/**
	 * MachineList - the Calm workspace machine collection (DESIGN.md §6.1).
	 * A readable card-row list, not a table: each row is identity (OS mark,
	 * name, subtitle), resources, a 7-state status pill and one labelled
	 * action, with a muted hint line when the state needs explaining. The
	 * collection ends on the allowance meter.
	 *
	 * Bulk actions stay available behind a "Select" toggle: calm by default,
	 * the power path one click away.
	 */
	import { getVmListContext, type VmListItem, type VmStatus } from './list.svelte';
	import { getVmBulkContext } from './bulk.svelte';
	import { displayStatus, type MachineDisplayStatus } from './display-status';
	import { compactBytes, rowAction, rowHint } from './machine-row';
	import MachineStatusPill from './MachineStatusPill.svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { getTaskOutcomeLedgerContext } from '$lib/features/tasks/task-outcome-ledger.svelte';
	import { getPowerActionsContext } from '$lib/features/tasks/power-actions.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';
	import { post } from '$lib/shared/api/client';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import ButtonLink from '$lib/shared/ui/ButtonLink.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
	import Toolbar from '$lib/shared/ui/Toolbar.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import OsMark from '$lib/shared/ui/OsMark.svelte';
	import AllowanceMeter from '$lib/shared/ui/AllowanceMeter.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';
	import SortButton from '$lib/shared/ui/SortButton.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import SearchIcon from '$lib/shared/ui/icons/SearchIcon.svelte';
	import ChevronDownIcon from '$lib/shared/ui/icons/ChevronDownIcon.svelte';

	const store = getVmListContext();
	const bulk = getVmBulkContext();
	const toast = getToastContext();
	const tray = getTaskTrayContext();
	const ledger = getTaskOutcomeLedgerContext();
	const powerActions = getPowerActionsContext();

	/** Allowances above this many slots render as a continuous bar. */
	const MAX_SEGMENTS = 12;

	/** A server status, or the client-derived "needs attention" pseudo-status. */
	type StatusFilter = VmStatus | '' | 'attention';

	const ATTENTION_VALUE = 'attention';

	/**
	 * The states "needs attention" keeps. All three are derived client-side
	 * (display-status.ts) - the server only knows running / stopped / paused -
	 * which is why the store loads every page when this filter is on.
	 */
	const ATTENTION_STATES: ReadonlySet<MachineDisplayStatus> = new Set([
		'failed',
		'partial',
		'provisioning'
	]);

	const STATUS_OPTIONS: readonly { value: StatusFilter; label: () => string }[] = [
		{ value: '', label: () => m['vms.list.filterAll']() },
		{ value: 'running', label: () => m['vms.list.filterRunning']() },
		{ value: 'stopped', label: () => m['vms.list.filterStopped']() },
		{ value: ATTENTION_VALUE, label: () => m['vms.list.filterAttention']() }
	];

	let selectMode = $state(false);

	function statusOf(machine: VmListItem): MachineDisplayStatus {
		return displayStatus(
			{ cluster: machine.cluster, vmid: machine.vmid, status: machine.status },
			{ tray: { tasks: tray.tasks }, ledger, inFlightAction: powerActions.get(machine.cluster, machine.vmid) }
		);
	}

	function detailHref(machine: VmListItem): string {
		return resolve('/vms/[cluster]/[vmid]', { cluster: machine.cluster, vmid: String(machine.vmid) });
	}

	/** Tags minus the internal marker; the cluster no longer rides here - it
	 *  is either the chip beside the name or implied by the active filter. */
	function visibleTags(machine: VmListItem): string[] {
		return machine.tags.filter((tag) => tag !== 'pvmss');
	}

	/**
	 * The list spans clusters whenever no cluster filter is active. Only then
	 * is a machine's name ambiguous: the same name and VMID can exist in two
	 * clusters, so the row carries the cluster as a chip and the link says
	 * which one it opens.
	 */
	const spansClusters = $derived(store.cluster === '');

	async function start(machine: VmListItem): Promise<void> {
		if (powerActions.get(machine.cluster, machine.vmid) !== null) return;
		powerActions.begin({ cluster: machine.cluster, vmid: machine.vmid, name: machine.name, action: 'start' });
		try {
			const result = await store.rowAction(machine.cluster, machine.vmid, 'start');
			if (result.ok) {
				toast.success(m['toast.vmStarted']({ name: machine.name }));
			} else {
				toast.error(m['toast.vmActionFailed']({ error: result.error ?? m['error.generic']() }));
			}
		} finally {
			powerActions.end(machine.cluster, machine.vmid);
		}
	}

	async function retry(): Promise<void> {
		try {
			await post('/api/v1/cluster/refresh');
		} catch {
			// The refresh itself may fail (cluster still down) - reload picks
			// up the current error state either way.
		}
		await store.load();
	}

	function toggleSelectMode(): void {
		selectMode = !selectMode;
		if (!selectMode) bulk.clear();
	}

	const allItems = $derived(store.result?.items ?? []);
	// "Needs attention" is a view-level filter over the full set the store
	// fetched; every other filter is already applied server-side.
	const items = $derived(
		store.attention ? allItems.filter((machine) => ATTENTION_STATES.has(statusOf(machine))) : allItems
	);

	function handleSelectAll(event: Event): void {
		const checked = (event.currentTarget as HTMLInputElement).checked;
		if (checked) bulk.selectPage(items);
		else bulk.clearPage(items);
	}

	const statusFilter = $derived<StatusFilter>(store.attention ? ATTENTION_VALUE : store.status);

	function setStatusFilter(value: string): void {
		if (value === ATTENTION_VALUE) store.setAttention(true);
		else store.setStatus(value as VmStatus | '');
	}

	const quota = $derived(store.result?.quota ?? null);
	const quotaFull = $derived(quota !== null && quota.allowed >= 0 && quota.used >= quota.allowed);
	// The attention filter has already fetched every page, so paginating the
	// result would page over a set that is complete.
	const pageCount = $derived(
		store.attention || store.result === null
			? 1
			: Math.max(1, Math.ceil(store.result.total / store.result.pageSize))
	);
	const filtered = $derived(
		store.search !== '' || store.status !== '' || store.node !== '' || store.attention
	);
	// Filtered lists show "N of M machines" - M is the owned-VM total from
	// quota.used, only present on the 'mine' scope; without it the plain
	// filtered count stands alone. The attention filter is client-side, so it
	// counts its own matches against the full total it fetched.
	const machineCountLabel = $derived.by(() => {
		const result = store.result;
		if (result === null) return '';
		if (store.attention) {
			return m['vms.list.machineCountFiltered']({ filtered: items.length, total: result.total });
		}
		if (filtered && result.quota !== undefined) {
			return m['vms.list.machineCountFiltered']({ filtered: result.total, total: result.quota.used });
		}
		return m['vms.list.machineCount']({ count: result.total });
	});
	const firstVisit = $derived(store.result?.emptyReason === 'no_vms_owned' && !filtered);
	const unreachable = $derived(store.errorCode === 'inventory_not_ready');
</script>

{#if quotaFull}
	<Alert tone="warning" role="status" class="mb-5" data-testid="vm-quota-full">
		<p class="font-medium">{m['vms.list.quotaFullTitle']()}</p>
		<p class="mt-0.5">{m['vms.list.quotaFullBody']()}</p>
	</Alert>
{/if}

{#if unreachable}
	<Card as="div" pad="none">
		<EmptyState
			title={m['vms.list.unreachableTitle']()}
			description={m['vms.list.unreachableBody']()}
			tone="error"
			dataTestid="vm-list-cluster-unreachable"
		>
			{#snippet actions()}
				<Button onclick={() => void retry()} data-testid="vm-list-cluster-retry">{m['vms.list.unreachableRetry']()}</Button>
			{/snippet}
		</EmptyState>
	</Card>
{:else if firstVisit}
	<Card as="div" pad="none" class="flex flex-col items-center gap-4 px-6 py-14 text-center" data-testid="vm-empty-owned">
		<div class="relative" aria-hidden="true">
			<span class="flex h-16 w-16 items-center justify-center rounded-lg border border-border bg-muted text-muted-foreground">
				<svg viewBox="0 0 24 24" class="h-7 w-7" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
					<rect x="2" y="3" width="20" height="14" rx="2" />
					<line x1="8" y1="21" x2="16" y2="21" />
					<line x1="12" y1="17" x2="12" y2="21" />
				</svg>
			</span>
			<span class="absolute -right-2 -top-2 flex h-6 w-6 items-center justify-center rounded-full bg-primary-solid text-sm font-semibold text-primary-foreground">+</span>
		</div>
		<p class="text-2xs font-semibold uppercase tracking-[0.08em] text-muted-foreground-subtle">{m['vms.list.emptyFirstEyebrow']()}</p>
		<div class="max-w-md">
			<p class="text-lg font-semibold text-foreground">{m['vms.list.emptyFirstTitle']()}</p>
			<p class="mt-1.5 text-sm text-muted-foreground">{m['vms.list.emptyFirstBody']()}</p>
		</div>
		<ButtonLink href={resolve('/vms/create')} data-testid="vm-empty-create">{m['vms.list.emptyFirstAction']()}</ButtonLink>
		<p class="text-xs text-muted-foreground-subtle">{m['vms.list.emptyFirstReassurance']()}</p>
	</Card>
{:else}
	<Card pad="none" class="machine-collection overflow-hidden" aria-label={m['vms.list.caption']()}>
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
					value={statusFilter}
					onchange={(event: Event) => setStatusFilter((event.currentTarget as HTMLSelectElement).value)}
					options={STATUS_OPTIONS.map((option) => ({ value: option.value, label: option.label() }))}
					data-testid="vm-status-filter"
				/>
			{/snippet}

			{#snippet meta()}
				{#if store.result}
					<span class="tabular-nums" data-testid="vm-count">{machineCountLabel}</span>
				{/if}
			{/snippet}

			{#snippet actions()}
				{#if items.length > 0}
					<Button
						variant="ghost"
						size="sm"
						aria-pressed={selectMode}
						title={m['vms.list.selectModeHint']()}
						onclick={toggleSelectMode}
						data-testid="vm-select-mode"
					>
						{selectMode ? m['vms.list.selectModeDone']() : m['vms.list.selectMode']()}
					</Button>
				{/if}
			{/snippet}
		</Toolbar>

		{#if store.error && !unreachable}
			<Alert data-testid="vm-list-error" class="m-4">{store.error}</Alert>
		{/if}

		{#if store.result === null && store.loading}
			<div class="flex flex-col" role="status" aria-live="polite" aria-label={m['common.loading']()} data-testid="vm-list-loading">
				{#each [0, 1, 2] as row (row)}
					<div class="flex items-center gap-4 border-b border-border-subtle px-4 py-4 last:border-b-0">
						<Skeleton class="h-[42px] w-[38px] rounded-lg" />
						<div class="flex flex-1 flex-col gap-2">
							<Skeleton class="h-4 w-40" />
							<Skeleton class="h-3 w-24" />
						</div>
						<Skeleton class="h-8 w-24" />
					</div>
				{/each}
			</div>
		{:else if store.result && items.length === 0}
			<EmptyState title={m['vms.list.noResultsTitle']()} description={m['vms.list.noResultsBody']()} dataTestid="vm-empty-match">
				{#snippet actions()}
					<Button variant="link" onclick={() => store.clearFilters()} data-testid="vm-clear-filters">{m['vms.list.clearFilters']()}</Button>
				{/snippet}
			</EmptyState>
		{:else if store.result}
			<div
				class="grid grid-cols-[minmax(0,1fr)_11rem_8.5rem_9rem] items-center gap-4 border-b border-border bg-muted/60 px-4 py-2 text-2xs font-semibold uppercase tracking-[0.04em] text-muted-foreground max-[699px]:hidden {selectMode
					? 'pl-12'
					: ''}"
				role="group"
				aria-label={m['vms.list.sortGroupLabel']()}
			>
				<SortButton
					label={m['vms.list.columnMachine']()}
					active={store.sortBy === 'name'}
					direction={store.sortDir}
					aria-label={m['vms.list.sortByName']()}
					onclick={() => store.setSort('name')}
					data-testid="vm-sort-name"
				/>
				<span>{m['vms.list.columnResources']()}</span>
				<SortButton
					label={m['vms.list.columnStatus']()}
					active={store.sortBy === 'status'}
					direction={store.sortDir}
					aria-label={m['vms.list.sortByStatus']()}
					onclick={() => store.setSort('status')}
					data-testid="vm-sort-status"
				/>
				<span class="sr-only">{m['vms.list.columnActions']()}</span>
			</div>
			{#if selectMode}
				<label class="flex items-center gap-3 border-b border-border px-4 py-2 text-xs text-muted-foreground">
					<input
						type="checkbox"
						class="h-4 w-4 rounded border-border accent-primary"
						checked={bulk.pageAllSelected(items)}
						onchange={handleSelectAll}
						data-testid="vm-bulk-select-all"
					/>
					{m['vms.list.selectAll']()}
				</label>
			{/if}
			<!-- The enclosing <section> already carries this label; repeating it
			     here makes assistive tech announce the same region twice. -->
			<ul class="flex flex-col">
				{#each items as machine (`${machine.cluster}:${machine.vmid}`)}
					{@const status = statusOf(machine)}
					{@const hint = rowHint(status)}
					{@const action = rowAction(status)}
					{@const busy = powerActions.get(machine.cluster, machine.vmid) !== null}
					<!-- The name link is stretched over the whole row (after:inset-0),
					     so a click anywhere on it opens the machine; the checkbox and
					     the action sit above the overlay (relative z-10). -->
					<li class="relative border-b border-border-subtle px-4 py-3.5 last:border-b-0 hover:bg-muted/40" data-testid="vm-row" data-status={status}>
						<div class="flex items-center gap-3">
							{#if selectMode}
								<input
									type="checkbox"
									class="relative z-10 h-4 w-4 shrink-0 rounded border-border accent-primary"
									checked={bulk.isSelected(machine.cluster, machine.vmid)}
									onchange={() => bulk.toggle({ cluster: machine.cluster, vmid: machine.vmid })}
									data-testid="vm-bulk-select-row"
									aria-label={m['vms.list.selectRow']({ name: machine.name })}
								/>
							{/if}
							<div class="grid min-w-0 flex-1 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 gap-y-2 min-[700px]:grid-cols-[minmax(0,1fr)_11rem_8.5rem_9rem]">
								<div class="flex min-w-0 items-center gap-3">
									<span class="max-[369px]:hidden"><OsMark ostype={machine.ostype} /></span>
									<div class="min-w-0">
										<div class="flex min-w-0 items-center gap-2">
											<a
												href={detailHref(machine)}
												class="pv-focus block min-w-0 truncate font-medium text-foreground underline-offset-2 after:absolute after:inset-0 after:content-[''] hover:text-primary hover:underline"
												aria-label={spansClusters
													? m['vms.list.rowLinkLabel']({
															name: machine.name,
															cluster: machine.clusterDisplayName
														})
													: undefined}
												data-testid="vm-row-link"
											>
												{machine.name}
											</a>
											{#if spansClusters}
												<span class="shrink-0" data-testid="vm-row-cluster">
													<Pill tone="off" dot={false} label={machine.clusterDisplayName} />
												</span>
											{/if}
										</div>
										{#if visibleTags(machine).length > 0}
											<p class="truncate text-xs text-muted-foreground">
												{visibleTags(machine).join(' · ')}
											</p>
										{/if}
									</div>
								</div>
								<p class="text-xs leading-5 text-muted-foreground max-[699px]:hidden">
									<span class="whitespace-nowrap font-mono tabular-nums">{m['vms.list.resourcesCompute']({ cpu: machine.cpuCores, memory: compactBytes(machine.memoryTotal) })}</span>
									<span class="block font-mono tabular-nums text-muted-foreground-subtle">VM {machine.vmid}</span>
								</p>
								<div class="max-[699px]:order-3">
									<MachineStatusPill {status} pending={busy || status === 'provisioning'} />
								</div>
								<div class="relative z-10 flex justify-end max-[699px]:order-4 max-[699px]:col-span-2 max-[699px]:justify-start max-[699px]:pl-[50px] max-[369px]:pl-0">
									{#if action === 'start'}
										<Button
											variant="secondary"
											size="sm"
											loading={busy}
											aria-label={m['vms.list.actionStartFor']({ name: machine.name })}
											onclick={() => void start(machine)}
											data-testid="vm-row-start"
										>
											{m['vms.list.actionStart']()}
										</Button>
									{:else}
										<ButtonLink
											variant="secondary"
											size="sm"
											href={detailHref(machine)}
											aria-label={m['vms.list.actionDetailsFor']({ name: machine.name })}
											data-testid="vm-row-details"
										>
											{m['vms.list.actionDetails']()}
										</ButtonLink>
									{/if}
								</div>
							</div>
						</div>
						{#if hint}
							<p
								class="mt-1.5 text-xs {hint.tone === 'error' ? 'text-destructive' : 'text-muted-foreground'} {selectMode ? 'pl-7' : ''} min-[370px]:pl-[50px]"
								data-testid="vm-row-hint"
							>
								{hint.text}
							</p>
						{/if}
					</li>
				{/each}
			</ul>

			{#if pageCount > 1}
				<nav class="flex items-center justify-end gap-2 border-t border-border px-4 py-2.5" aria-label={m['vms.list.paginationLabel']()}>
					<span class="font-mono text-xs tabular-nums text-muted-foreground" data-testid="vm-page-indicator">
						{m['common.pageIndicator']({ current: store.result.page, total: pageCount })}
					</span>
					<Button
						variant="secondary"
						size="icon-sm"
						label={m['common.previous']()}
						disabled={store.result.page <= 1}
						onclick={() => store.setPage(store.page - 1)}
						data-testid="vm-page-prev"
					>
						<ChevronDownIcon class="h-4 w-4 rotate-90" />
					</Button>
					<Button
						variant="secondary"
						size="icon-sm"
						label={m['common.next']()}
						disabled={store.result.page >= pageCount}
						onclick={() => store.setPage(store.page + 1)}
						data-testid="vm-page-next"
					>
						<ChevronDownIcon class="h-4 w-4 -rotate-90" />
					</Button>
				</nav>
			{/if}
		{/if}

		{#if quota}
			<div class="flex flex-wrap items-center gap-x-4 gap-y-2 border-t border-border bg-muted/40 px-4 py-3 text-xs text-muted-foreground" data-testid="vm-quota">
				{#if quota.allowed >= 0}
					<span class="font-medium text-foreground tabular-nums">{m['vms.list.allowanceUsed']({ used: quota.used, allowed: quota.allowed })}</span>
					<div class="w-40">
						{#if quota.allowed > 0 && quota.allowed <= MAX_SEGMENTS}
							<AllowanceMeter used={quota.used} limit={quota.allowed} label={m['vms.list.allowanceMeterLabel']({ used: quota.used, allowed: quota.allowed })} />
						{:else}
							<div
								class="h-1.5 w-full overflow-hidden rounded-full bg-muted"
								role="meter"
								aria-valuenow={Math.min(quota.used, quota.allowed)}
								aria-valuemin={0}
								aria-valuemax={quota.allowed}
								aria-label={m['vms.list.allowanceMeterLabel']({ used: quota.used, allowed: quota.allowed })}
							>
								<div class="h-full rounded-full bg-primary" style="width: {quota.allowed === 0 ? 100 : Math.min(100, (quota.used / quota.allowed) * 100)}%"></div>
							</div>
						{/if}
					</div>
				{:else}
					<span class="font-medium text-foreground tabular-nums">{m['vms.list.allowanceUnlimited']({ used: quota.used })}</span>
				{/if}
				<span class="ml-auto">{m['vms.list.allowanceSource']()}</span>
			</div>
		{/if}
	</Card>
{/if}

{#if !unreachable}
	<aside class="mt-6 flex flex-wrap items-center gap-3 px-1 text-sm text-muted-foreground" data-testid="vm-quiet-help">
		<svg viewBox="0 0 24 24" class="h-4 w-4 shrink-0" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
			<path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
			<path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" />
		</svg>
		<span class="font-medium text-foreground">{m['vms.list.quietHelpTitle']()}</span>
		<span class="max-[699px]:hidden">{m['vms.list.quietHelpBody']()}</span>
		<a href={resolve('/docs')} class="pv-focus rounded font-medium text-primary underline-offset-2 hover:underline">{m['vms.list.openGuide']()}</a>
	</aside>
{/if}
