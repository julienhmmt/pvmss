<script lang="ts">
	/**
	 * AdminVmsPage - the admin-wide machine collection: every PVMSS VM of
	 * every pool, across every cluster, with the power actions the owner has
	 * on their own detail page. Rows open the same detail view (rendered
	 * under /admin/vms so the back link returns here instead of the user
	 * workspace).
	 *
	 * State comes from the shared VmListStore (scope 'all'), so search, sort,
	 * filters, pagination, and the optimistic power flip follow the same code
	 * path as the user's list.
	 */
	import { resolve } from '$app/paths';
	import { getVmListContext, type VmListItem, type VmStatus } from '#lib/features/vms/list.svelte.js';
	import { displayStatus, type MachineDisplayStatus } from '#lib/features/vms/display-status.js';
	import { compactBytes } from '#lib/features/vms/machine-row.js';
	import MachineStatusPill from '#lib/features/vms/MachineStatusPill.svelte';
	import type { VmAction } from '#lib/features/vms/detail.svelte.js';
	import { getTaskTrayContext } from '#lib/features/tasks/tasks.svelte.js';
	import { getTaskOutcomeLedgerContext } from '#lib/features/tasks/task-outcome-ledger.svelte.js';
	import { getPowerActionsContext } from '#lib/features/tasks/power-actions.svelte.js';
	import { getToastContext } from '#lib/shared/ui/toast.svelte.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import ButtonLink from '#lib/shared/ui/ButtonLink.svelte';
	import ConfirmDialog from '#lib/shared/ui/ConfirmDialog.svelte';
	import EmptyState from '#lib/shared/ui/EmptyState.svelte';
	import OsMark from '#lib/shared/ui/OsMark.svelte';
	import Pill from '#lib/shared/ui/Pill.svelte';
	import Select from '#lib/shared/ui/Select.svelte';
	import TableCard from '#lib/shared/ui/TableCard.svelte';
	import TableHeader from '#lib/shared/ui/TableHeader.svelte';
	import TableSkeleton from '#lib/shared/ui/TableSkeleton.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';
	import SearchIcon from '#lib/shared/ui/icons/SearchIcon.svelte';
	import ChevronDownIcon from '#lib/shared/ui/icons/ChevronDownIcon.svelte';
	import { m } from '#lib/paraglide/messages.js';

	const store = getVmListContext();
	const toast = getToastContext();
	const tray = getTaskTrayContext();
	const ledger = getTaskOutcomeLedgerContext();
	const powerActions = getPowerActionsContext();

	const PAGE_SIZES: readonly number[] = [10, 25, 50];

	const STATUS_OPTIONS: readonly { value: VmStatus | ''; label: () => string }[] = [
		{ value: '', label: () => m['vms.list.filterAll']() },
		{ value: 'running', label: () => m['vms.list.filterRunning']() },
		{ value: 'stopped', label: () => m['vms.list.filterStopped']() },
		{ value: 'paused', label: () => m['machine.status.paused']() }
	];

	/** The power actions offered per row (force stop and reset confirm first). */
	type QuickAction = Extract<VmAction, 'start' | 'stop' | 'reset'>;

	/** The destructive action awaiting confirmation, if any. */
	type PendingAction = { action: Extract<QuickAction, 'stop' | 'reset'>; machine: VmListItem };
	let pending = $state<PendingAction | null>(null);

	const items = $derived(store.result?.items ?? []);
	const poolOptions = $derived([
		{ value: '', label: m['admin.vms.filterPoolAll']() },
		...(store.result?.availablePools ?? []).map((pool) => ({ value: pool, label: pool }))
	]);
	const pageSizeOptions = $derived(
		(PAGE_SIZES.includes(store.pageSize) ? PAGE_SIZES : [store.pageSize, ...PAGE_SIZES]).map((size) => ({
			value: String(size),
			label: m['vms.list.perPage']({ count: size })
		}))
	);
	const pageCount = $derived(
		store.result === null ? 1 : Math.max(1, Math.ceil(store.result.total / store.result.pageSize))
	);
	const countLabel = $derived.by(() => {
		const result = store.result;
		if (result === null) return '';
		if (store.filtered) {
			return m['admin.vms.countFiltered']({ filtered: result.total });
		}
		return m['admin.vms.count']({ count: result.total });
	});
	const filtered = $derived(store.filtered);

	function statusOf(machine: VmListItem): MachineDisplayStatus {
		return displayStatus(
			{ cluster: machine.cluster, vmid: machine.vmid, status: machine.status },
			{ tray: { tasks: tray.tasks }, ledger, inFlightAction: powerActions.get(machine.cluster, machine.vmid) }
		);
	}

	function detailHref(machine: VmListItem): string {
		return resolve('/admin/vms/[cluster]/[vmid]', { cluster: machine.cluster, vmid: String(machine.vmid) });
	}

	function consoleHref(machine: VmListItem): string {
		return resolve('/vms/[cluster]/[vmid]/console', { cluster: machine.cluster, vmid: String(machine.vmid) });
	}

	function visibleTags(machine: VmListItem): string[] {
		return machine.tags.filter((tag) => tag !== 'pvmss');
	}

	const spansClusters = $derived(store.cluster === '');

	function busy(machine: VmListItem): boolean {
		return powerActions.get(machine.cluster, machine.vmid) !== null;
	}

	async function run(machine: VmListItem, action: QuickAction): Promise<void> {
		if (busy(machine)) return;
		powerActions.begin({ cluster: machine.cluster, vmid: machine.vmid, name: machine.name, action });
		try {
			const result = await store.rowAction(machine.cluster, machine.vmid, action);
			if (result.ok) {
				toast.success(successToast(action, machine.name));
			} else {
				toast.error(m['toast.vmActionFailed']({ error: result.error ?? m['error.generic']() }));
			}
		} finally {
			powerActions.end(machine.cluster, machine.vmid);
		}
	}

	function successToast(action: QuickAction, name: string): string {
		if (action === 'stop') return m['toast.vmStopped']({ name });
		if (action === 'reset') return m['toast.vmReset']({ name });
		return m['toast.vmStarted']({ name });
	}

	/** Forced power cuts ask first; start does not. */
	function request(machine: VmListItem, action: QuickAction): void {
		if (action === 'stop' || action === 'reset') {
			pending = { action, machine };
			return;
		}
		void run(machine, action);
	}

	function confirmPending(): void {
		if (pending === null) return;
		const { action, machine } = pending;
		pending = null;
		void run(machine, action);
	}

	const confirmTitle = $derived(
		pending === null
			? ''
			: pending.action === 'stop'
				? m['vms.confirm.forceStop.title']({ name: pending.machine.name })
				: m['vms.confirm.reset.title']({ name: pending.machine.name })
	);
	const confirmMessage = $derived(
		pending === null
			? ''
			: pending.action === 'stop'
				? m['vms.confirm.forceStop.message']()
				: m['vms.confirm.reset.message']()
	);
	const confirmLabel = $derived(
		pending === null
			? ''
			: pending.action === 'stop'
				? m['vms.confirm.forceStop.confirm']()
				: m['vms.confirm.reset.confirm']()
	);

	async function retry(): Promise<void> {
		await store.load();
	}
</script>

{#if store.error}
	<Alert data-testid="admin-vm-list-error" class="mb-4">
		{store.error}
	</Alert>
{/if}

{#if store.result === null && store.loading}
	<TableSkeleton columns={6} />
{:else if store.result && items.length === 0}
	<TableCard>
		<EmptyState
			title={filtered ? m['admin.vms.noMatchTitle']() : m['admin.vms.emptyTitle']()}
			description={filtered ? m['admin.vms.noMatchBody']() : m['admin.vms.emptyBody']()}
			dataTestid="admin-vm-empty"
		>
			{#snippet actions()}
				{#if filtered}
					<Button variant="secondary" onclick={() => store.clearFilters()} data-testid="admin-vm-clear-filters">
						{m['admin.vms.clearFilters']()}
					</Button>
				{:else}
					<Button variant="secondary" onclick={() => void retry()}>{m['common.refresh']()}</Button>
				{/if}
			{/snippet}
		</EmptyState>
	</TableCard>
{:else if store.result}
	<TableCard>
		{#snippet toolbar()}
			<label for="admin-vm-search" class="sr-only">{m['common.search']()}</label>
			<TextField
				id="admin-vm-search"
				type="search"
				class="w-full sm:max-w-xs"
				placeholder={m['admin.vms.searchPlaceholder']()}
				value={store.search}
				oninput={(event: Event) => store.applySearch((event.currentTarget as HTMLInputElement).value)}
				data-testid="admin-vm-search"
			>
				{#snippet leading()}<SearchIcon class="h-4 w-4" />{/snippet}
			</TextField>
			<label for="admin-vm-pool-filter" class="sr-only">{m['admin.vms.filterPool']()}</label>
			<Select
				id="admin-vm-pool-filter"
				class="w-auto min-w-[9rem]"
				value={store.pool}
				options={poolOptions}
				onchange={(event: Event) => store.setPool((event.currentTarget as HTMLSelectElement).value)}
				data-testid="admin-vm-pool-filter"
			/>
			<label for="admin-vm-status-filter" class="sr-only">{m['vms.list.filterStatusLabel']()}</label>
			<Select
				id="admin-vm-status-filter"
				class="w-auto min-w-[9rem]"
				value={store.status}
				options={STATUS_OPTIONS.map((option) => ({ value: option.value, label: option.label() }))}
				onchange={(event: Event) => store.setStatus((event.currentTarget as HTMLSelectElement).value as VmStatus | '')}
				data-testid="admin-vm-status-filter"
			/>
			<span class="ml-auto text-xs tabular-nums text-muted-foreground" data-testid="admin-vm-count">{countLabel}</span>
		{/snippet}

		<table class="pv-table pv-responsive-table">
			<caption class="sr-only">{m['admin.vms.tableCaption']()}</caption>
			<thead>
				<tr>
					<TableHeader
						text={m['vms.list.columnMachine']()}
						column="name"
						activeColumn={store.sortBy}
						sortDir={store.sortDir}
						onSort={(column) => store.setSort(column)}
					/>
					<TableHeader text={m['vms.list.columnPool']()} />
					<TableHeader
						text={m['vms.list.columnNode']()}
						column="node"
						activeColumn={store.sortBy}
						sortDir={store.sortDir}
						onSort={(column) => store.setSort(column)}
					/>
					<TableHeader
						text={m['vms.list.columnStatus']()}
						column="status"
						activeColumn={store.sortBy}
						sortDir={store.sortDir}
						onSort={(column) => store.setSort(column)}
					/>
					<TableHeader text={m['vms.list.columnResources']()} />
					<TableHeader text={m['vms.list.columnActions']()} class="text-right" />
				</tr>
			</thead>
			<tbody>
				{#each items as machine (`${machine.cluster}:${machine.vmid}`)}
					{@const status = statusOf(machine)}
					{@const inFlight = busy(machine)}
					<tr class="relative" data-testid="admin-vm-row" data-status={status}>
						<td data-label={m['vms.list.columnMachine']()}>
							<div class="flex min-w-0 items-center gap-3">
								<span class="max-[369px]:hidden"><OsMark ostype={machine.ostype} /></span>
								<div class="min-w-0">
									<div class="flex min-w-0 items-center gap-2">
										<a
											href={detailHref(machine)}
											class="pv-focus block max-w-full shrink-0 truncate font-medium text-foreground underline-offset-2 after:absolute after:inset-0 after:content-[''] hover:text-primary hover:underline"
											aria-label={spansClusters
												? m['vms.list.rowLinkLabel']({
														name: machine.name,
														cluster: machine.clusterDisplayName
													})
												: m['vms.list.actionDetailsFor']({ name: machine.name })}
											data-testid="admin-vm-row-link"
										>
											{machine.name}
										</a>
										{#if spansClusters}
											<span class="min-w-0 shrink overflow-hidden" data-testid="admin-vm-row-cluster">
												<Pill tone="off" dot={false} label={machine.clusterDisplayName} />
											</span>
										{/if}
									</div>
									<p class="truncate font-mono text-xs tabular-nums text-muted-foreground-subtle">
										VM {machine.vmid}{#if visibleTags(machine).length > 0}
											· {visibleTags(machine).join(' · ')}{/if}
									</p>
								</div>
							</div>
						</td>
						<td data-label={m['vms.list.columnPool']()} data-testid="admin-vm-pool">
							{#if machine.pool === ''}
								<span class="text-xs text-muted-foreground-subtle">{m['admin.vms.unpooled']()}</span>
							{:else}
								<span class="font-mono text-xs">{machine.pool}</span>
							{/if}
						</td>
						<td data-label={m['vms.list.columnNode']()}>
							<span class="font-mono text-xs">{machine.node}</span>
						</td>
						<td data-label={m['vms.list.columnStatus']()}>
							<MachineStatusPill {status} pending={inFlight || status === 'provisioning'} />
						</td>
						<td data-label={m['vms.list.columnResources']()}>
							<span class="whitespace-nowrap font-mono text-xs tabular-nums">
								{m['vms.list.resourcesCompute']({
									cpu: machine.cpuCores,
									memory: compactBytes(machine.memoryTotal)
								})}
							</span>
						</td>
						<td data-label={m['vms.list.columnActions']()} data-nolabel="true" class="relative z-10">
							<div class="flex flex-wrap items-center justify-end gap-2">
								{#if machine.status === 'running' && !inFlight}
									<ButtonLink
										variant="secondary"
										size="sm"
										href={consoleHref(machine)}
										target="_blank"
										rel="noopener noreferrer"
										aria-label={m['vms.list.actionConsoleFor']({ name: machine.name })}
										data-testid="admin-vm-console"
									>
										{m['vms.list.actionConsole']()}
									</ButtonLink>
								{/if}
								{#if machine.status === 'stopped'}
									<Button
										variant="secondary"
										size="sm"
										loading={inFlight}
										aria-label={m['vms.list.actionStartFor']({ name: machine.name })}
										onclick={() => request(machine, 'start')}
										data-testid="admin-vm-start"
									>
										{m['vms.action.start']()}
									</Button>
								{:else}
									<Button
										variant="destructive"
										size="sm"
										disabled={inFlight}
										aria-label={m['admin.vms.actionStopFor']({ name: machine.name })}
										onclick={() => request(machine, 'stop')}
										data-testid="admin-vm-stop"
									>
										{m['vms.action.stop']()}
									</Button>
									<Button
										variant="secondary"
										size="sm"
										disabled={inFlight}
										aria-label={m['admin.vms.actionResetFor']({ name: machine.name })}
										onclick={() => request(machine, 'reset')}
										data-testid="admin-vm-reset"
									>
										{m['vms.action.reset']()}
									</Button>
								{/if}
								<ButtonLink
									variant="ghost"
									size="sm"
									href={detailHref(machine)}
									aria-label={m['vms.list.actionDetailsFor']({ name: machine.name })}
									data-testid="admin-vm-details"
								>
									{m['vms.list.actionDetails']()}
								</ButtonLink>
							</div>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>

		{#if pageCount > 1}
			<nav
				class="flex items-center justify-end gap-2 border-t border-border px-4 py-2.5"
				aria-label={m['vms.list.paginationLabel']()}
			>
				<label for="admin-vm-page-size" class="sr-only">{m['vms.list.pageSizeLabel']()}</label>
				<Select
					id="admin-vm-page-size"
					class="w-auto"
					value={String(store.pageSize)}
					onchange={(event: Event) => store.setPageSize(Number((event.currentTarget as HTMLSelectElement).value))}
					options={pageSizeOptions}
					data-testid="admin-vm-page-size"
				/>
				<span class="font-mono text-xs tabular-nums text-muted-foreground" data-testid="admin-vm-page-indicator">
					{m['common.pageIndicator']({ current: store.result.page, total: pageCount })}
				</span>
				<Button
					variant="secondary"
					size="icon-sm"
					label={m['common.previous']()}
					disabled={store.result.page <= 1}
					onclick={() => store.setPage(store.page - 1)}
					data-testid="admin-vm-page-prev"
				>
					<ChevronDownIcon class="h-4 w-4 rotate-90" />
				</Button>
				<Button
					variant="secondary"
					size="icon-sm"
					label={m['common.next']()}
					disabled={store.result.page >= pageCount}
					onclick={() => store.setPage(store.page + 1)}
					data-testid="admin-vm-page-next"
				>
					<ChevronDownIcon class="h-4 w-4 -rotate-90" />
				</Button>
			</nav>
		{/if}
	</TableCard>
{/if}

<ConfirmDialog
	open={pending !== null}
	title={confirmTitle}
	message={confirmMessage}
	confirmLabel={confirmLabel}
	cancelLabel={m['common.cancel']()}
	onConfirm={confirmPending}
	onClose={() => (pending = null)}
/>
