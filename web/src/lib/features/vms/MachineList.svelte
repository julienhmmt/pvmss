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
	import { getVmListContext, type VmListItem } from './list.svelte';
	import { getVmBulkContext } from './bulk.svelte';
	import { displayStatus, type MachineDisplayStatus } from './display-status';
	import MachineRow from './MachineRow.svelte';
	import MachineListToolbar from './MachineListToolbar.svelte';
	import { getTaskTrayContext } from '#lib/features/tasks/tasks.svelte.js';
	import { getTaskOutcomeLedgerContext } from '#lib/features/tasks/task-outcome-ledger.svelte.js';
	import { getPowerActionsContext } from '#lib/features/tasks/power-actions.svelte.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import { getToastContext } from '#lib/shared/ui/toast.svelte.js';
	import { resolve } from '$app/paths';
	import { m } from '#lib/paraglide/messages.js';
	import { post } from '#lib/shared/api/client.js';
	import EmptyState from '#lib/shared/ui/EmptyState.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import ButtonLink from '#lib/shared/ui/ButtonLink.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import Select from '#lib/shared/ui/Select.svelte';
	import AllowanceMeter from '#lib/shared/ui/AllowanceMeter.svelte';
	import Skeleton from '#lib/shared/ui/Skeleton.svelte';
	import ChevronDownIcon from '#lib/shared/ui/icons/ChevronDownIcon.svelte';

	import { getSessionContext } from '#lib/features/auth/session.svelte.js';

	const store = getVmListContext();
	const bulk = getVmBulkContext();
	const toast = getToastContext();
	const tray = getTaskTrayContext();
	const ledger = getTaskOutcomeLedgerContext();
	const powerActions = getPowerActionsContext();
	const session = getSessionContext();

	/** Allowances above this many slots render as a continuous bar. */
	const MAX_SEGMENTS = 12;

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

	let selectMode = $state(false);
	/** True while "select all matching" is fetching every page. */
	let selectingAll = $state(false);

	/** Page sizes offered in the pagination footer. */
	const PAGE_SIZES: readonly number[] = [10, 25, 50];

	function statusOf(machine: VmListItem): MachineDisplayStatus {
		return displayStatus(
			{ cluster: machine.cluster, vmid: machine.vmid, status: machine.status },
			{ tray: { tasks: tray.tasks }, ledger, inFlightAction: powerActions.get(machine.cluster, machine.vmid) }
		);
	}

	/**
	 * The list spans clusters whenever no cluster filter is active. Only then
	 * is a machine's name ambiguous: the same name and VMID can exist in two
	 * clusters, so the row carries the cluster as a chip and the link says
	 * which one it opens.
	 */

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
	// The chip only tells rows apart when the list mixes clusters; a user
	// pinned to one cluster sees its name in the sidebar already.
	const spansClusters = $derived(store.cluster === '' && new Set(allItems.map((item) => item.cluster)).size > 1);
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

	// Page selection only covers what is loaded; when the filtered set
	// spans pages, offer a way to select every match (the attention filter
	// has already fetched all pages, so it never needs this).
	const moreToSelect = $derived(
		!store.attention && bulk.selectedCount < (store.result?.total ?? 0) && items.length < (store.result?.total ?? 0)
	);

	async function handleSelectAllMatching(): Promise<void> {
		selectingAll = true;
		try {
			bulk.selectPage(await store.fetchAllMatching());
		} catch {
			toast.error(m['vms.list.errorLoading']());
		} finally {
			selectingAll = false;
		}
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
	const filtered = $derived(store.filtered);
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
	// A machine being created is not in the inventory yet: say so rather than
	// invite a first create, which would make a duplicate.
	const creating = $derived(tray.tasks.filter((task) => task.kind === 'vm_create'));
	const firstVisit = $derived(store.noneOwned && creating.length === 0);
	const provisioningOnly = $derived(store.noneOwned && creating.length > 0);
	const unreachable = $derived(store.errorCode === 'inventory_not_ready');
	// A pageSize arriving via the URL may not be one of the offered choices;
	// surface it rather than let the select misreport the active size.
	const pageSizeOptions = $derived.by(() => {
		const sizes = PAGE_SIZES.includes(store.pageSize) ? PAGE_SIZES : [store.pageSize, ...PAGE_SIZES];
		return sizes.map((size) => ({ value: String(size), label: m['vms.list.perPage']({ count: size }) }));
	});
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
{:else if provisioningOnly}
	<Card as="div" pad="none" class="flex flex-col items-center gap-2 px-6 py-14 text-center" data-testid="vm-empty-provisioning" role="status">
		{#each creating as task (task.upid)}
			<p class="text-lg font-semibold text-foreground">{m['vms.list.provisioningTitle']({ name: task.name })}</p>
		{/each}
		<p class="max-w-md text-sm text-muted-foreground">{m['vms.list.provisioningBody']()}</p>
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
		<div class="max-w-md">
			<p class="text-lg font-semibold text-foreground">{session.isAdmin ? m['vms.list.emptyFirstAdminTitle']() : m['vms.list.emptyFirstTitle']()}</p>
			<p class="mt-1.5 text-sm text-muted-foreground">{session.isAdmin ? m['vms.list.emptyFirstAdminBody']() : m['vms.list.emptyFirstBody']()}</p>
		</div>
		{#if !session.isAdmin}
			<ButtonLink href={resolve('/vms/create')} data-testid="vm-empty-create">{m['vms.list.emptyFirstAction']()}</ButtonLink>
		{/if}
	</Card>
{:else}
	<Card pad="none" class="machine-collection overflow-hidden" aria-label={m['vms.list.caption']()}>
		<MachineListToolbar
			selectMode={selectMode}
			showToggle={items.length > 0}
			countLabel={machineCountLabel}
			onToggleSelectMode={toggleSelectMode}
		/>

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
			<!-- Caption row only - sorting lives in the toolbar's single
			     "Sort by" control. The select-mode left padding mirrors the
			     row's checkbox (1rem box + 0.75rem gap inside px-4). -->
			<div
				class="grid grid-cols-[minmax(0,1fr)_11rem_9rem_12.5rem] items-center gap-4 border-b border-border bg-muted/60 px-4 py-2 text-2xs font-semibold uppercase tracking-[0.04em] text-muted-foreground max-[699px]:hidden {selectMode
					? 'pl-11'
					: ''}"
			>
				<span>{m['vms.list.columnMachine']()}</span>
				<span>{m['vms.list.columnResources']()}</span>
				<span>{m['vms.list.columnStatus']()}</span>
				<span class="sr-only">{m['vms.list.columnActions']()}</span>
			</div>
			{#if selectMode}
				<div class="flex flex-wrap items-center gap-3 border-b border-border px-4 py-2 text-xs text-muted-foreground">
					<label class="flex items-center gap-3">
						<input
							type="checkbox"
							class="h-4 w-4 rounded border-border accent-primary"
							checked={bulk.pageAllSelected(items)}
							onchange={handleSelectAll}
							data-testid="vm-bulk-select-all"
						/>
						{m['vms.list.selectAll']()}
					</label>
					{#if moreToSelect}
						<Button
							variant="link"
							loading={selectingAll}
							onclick={() => void handleSelectAllMatching()}
							data-testid="vm-bulk-select-all-matching"
						>
							{m['vms.list.selectAllMatching']({ total: store.result?.total ?? 0 })}
						</Button>
					{/if}
				</div>
			{/if}
			<!-- The enclosing <section> already carries this label; repeating it
			     here makes assistive tech announce the same region twice. -->
			<ul class="flex flex-col">
				{#each items as machine (`${machine.cluster}:${machine.vmid}`)}
					{@const status = statusOf(machine)}
					<MachineRow
						{machine}
						{status}
						busy={powerActions.get(machine.cluster, machine.vmid) !== null}
						{selectMode}
						{spansClusters}
						onStart={(target) => void start(target)}
					/>
				{/each}
			</ul>

			{#if pageCount > 1}
				<nav class="flex items-center justify-end gap-2 border-t border-border px-4 py-2.5" aria-label={m['vms.list.paginationLabel']()}>
					<label for="vm-page-size" class="sr-only">{m['vms.list.pageSizeLabel']()}</label>
					<Select
						id="vm-page-size"
						class="w-auto"
						value={String(store.pageSize)}
						onchange={(event: Event) => store.setPageSize(Number((event.currentTarget as HTMLSelectElement).value))}
						options={pageSizeOptions}
						data-testid="vm-page-size"
					/>
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
	<div class="mt-6 flex flex-wrap items-center gap-3 px-1 text-sm text-muted-foreground" data-testid="vm-quiet-help">
		<svg viewBox="0 0 24 24" class="h-4 w-4 shrink-0" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
			<path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
			<path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" />
		</svg>
		<span class="font-medium text-foreground">{m['vms.list.quietHelpTitle']()}</span>
		<span class="max-[699px]:hidden">{m['vms.list.quietHelpBody']()}</span>
		<ButtonLink href={resolve('/docs')} variant="secondary" size="sm">{m['vms.list.openGuide']()}</ButtonLink>
	</div>
{/if}
