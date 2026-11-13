<script lang="ts">
	/**
	 * SearchPage - a navigator table: one query field, one continuous list of
	 * matching machines across clusters. The field keeps focus while ↑/↓ move
	 * a row selection (aria-activedescendant) and Enter opens the machine;
	 * reaching the last row or scrolling to the bottom fetches the next page.
	 * Matched text is marked in name, tags and (exact-match only) VMID, the
	 * same rules the server applies in searchVMs.
	 */
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { m } from '#lib/paraglide/messages.js';
	import { getSearchContext } from './search.svelte';
	import { highlightText, vmidMatches } from './highlight';
	import type { VmListItem, VmStatus } from '#lib/features/vms/list.svelte.js';
	import { displayStatus, type MachineDisplayStatus } from '#lib/features/vms/display-status.js';
	import { compactBytes } from '#lib/features/vms/machine-row.js';
	import type { VmAction } from '#lib/features/vms/detail.svelte.js';
	import { getTaskTrayContext } from '#lib/features/tasks/tasks.svelte.js';
	import { getTaskOutcomeLedgerContext } from '#lib/features/tasks/task-outcome-ledger.svelte.js';
	import { getPowerActionsContext } from '#lib/features/tasks/power-actions.svelte.js';
	import { getToastContext } from '#lib/shared/ui/toast.svelte.js';
	import MachineStatusPill from '#lib/features/vms/MachineStatusPill.svelte';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import ButtonLink from '#lib/shared/ui/ButtonLink.svelte';
	import ClusterSelector from '#lib/shared/ui/ClusterSelector.svelte';
	import ConfirmDialog from '#lib/shared/ui/ConfirmDialog.svelte';
	import EmptyState from '#lib/shared/ui/EmptyState.svelte';
	import OsMark from '#lib/shared/ui/OsMark.svelte';
	import PageHeader from '#lib/shared/ui/PageHeader.svelte';
	import Select from '#lib/shared/ui/Select.svelte';
	import TableCard from '#lib/shared/ui/TableCard.svelte';
	import TableHeader from '#lib/shared/ui/TableHeader.svelte';
	import TableSkeleton from '#lib/shared/ui/TableSkeleton.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';
	import SearchIcon from '#lib/shared/ui/icons/SearchIcon.svelte';

	const store = getSearchContext();
	const toast = getToastContext();
	const tray = getTaskTrayContext();
	const ledger = getTaskOutcomeLedgerContext();
	const powerActions = getPowerActionsContext();

	type QuickAction = Extract<VmAction, 'start' | 'stop' | 'reset'>;
	type PendingAction = { action: Extract<QuickAction, 'stop' | 'reset'>; machine: VmListItem };

	const items = $derived(store.result?.items ?? []);
	const spansClusters = $derived(store.cluster === '' && store.clusterOptions.length > 1);
	const matchQuery = $derived(store.matchQuery);

	let selectedIndex = $state(-1);
	let pending = $state<PendingAction | null>(null);
	let sentinel = $state<HTMLElement | null>(null);

	const statusOptions = $derived([
		{ value: '', label: m['common.all']() },
		{ value: 'running', label: m['machine.status.running']() },
		{ value: 'stopped', label: m['machine.status.stopped']() },
		{ value: 'paused', label: m['machine.status.paused']() }
	]);

	/** A fresh first page selects the first row so Enter always has a target;
	 *  appended pages keep the selection where the user left it. */
	$effect(() => {
		if (store.result?.page === 1) selectedIndex = items.length > 0 ? 0 : -1;
	});

	/** The observer fires only on intersection changes, so after an append
	 *  that leaves the sentinel in view we re-check by hand until it scrolls
	 *  out of the viewport. */
	$effect(() => {
		const el = sentinel;
		if (el === null) return;
		const observer = new IntersectionObserver((entries) => {
			if (entries.some((entry) => entry.isIntersecting)) maybeLoadMore();
		});
		observer.observe(el);
		return () => observer.disconnect();
	});

	function maybeLoadMore(): void {
		if (!store.hasMore || store.loadingMore) return;
		void store.loadMore().then(() => {
			const el = sentinel;
			if (el !== null && el.getBoundingClientRect().top < window.innerHeight) maybeLoadMore();
		});
	}

	function vmHref(machine: VmListItem): string {
		return resolve('/vms/[cluster]/[vmid]', { cluster: encodeURIComponent(machine.cluster), vmid: String(machine.vmid) });
	}

	function consoleHref(machine: VmListItem): string {
		return resolve('/vms/[cluster]/[vmid]/console', { cluster: machine.cluster, vmid: String(machine.vmid) });
	}

	function visibleTags(machine: VmListItem): string[] {
		return machine.tags.filter((tag) => tag !== 'pvmss');
	}

	function statusOf(machine: VmListItem): MachineDisplayStatus {
		return displayStatus(
			{ cluster: machine.cluster, vmid: machine.vmid, status: machine.status },
			{ tray: { tasks: tray.tasks }, ledger, inFlightAction: powerActions.get(machine.cluster, machine.vmid) }
		);
	}

	function busy(machine: VmListItem): boolean {
		return powerActions.get(machine.cluster, machine.vmid) !== null;
	}

	function handleInput(event: Event): void {
		store.applySearch((event.currentTarget as HTMLInputElement).value);
	}

	function handleStatusChange(event: Event): void {
		store.setStatus((event.currentTarget as HTMLSelectElement).value as '' | VmStatus);
	}

	/** ↑/↓ walk the selection, Enter opens it. The field keeps focus, so
	 *  typing and navigating never compete for it. */
	function handleKeydown(event: KeyboardEvent): void {
		if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
			event.preventDefault();
			if (items.length === 0) return;
			const delta = event.key === 'ArrowDown' ? 1 : -1;
			selectedIndex = Math.min(Math.max(selectedIndex + delta, 0), items.length - 1);
			if (selectedIndex === items.length - 1) maybeLoadMore();
			document.getElementById(`search-result-${selectedIndex}`)?.scrollIntoView({ block: 'nearest' });
			return;
		}
		if (event.key === 'Enter') {
			const machine = items[selectedIndex];
			if (machine === undefined) return;
			event.preventDefault();
			void goto(vmHref(machine));
		}
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
		pending === null ? '' : pending.action === 'stop' ? m['vms.confirm.forceStop.message']() : m['vms.confirm.reset.message']()
	);
	const confirmLabel = $derived(
		pending === null ? '' : pending.action === 'stop' ? m['vms.confirm.forceStop.confirm']() : m['vms.confirm.reset.confirm']()
	);
</script>

<section class="w-full">
	<PageHeader eyebrow={m['chrome.context.workspace']()} title={m['search.heading']()} focusTarget />

	<div class="mb-2 flex flex-col gap-3 sm:flex-row sm:items-center">
		<div class="min-w-0 flex-1">
			<label for="global-search" class="sr-only">{m['search.label']()}</label>
			<TextField
				id="global-search"
				type="search"
				size="lg"
				placeholder={m['search.placeholder']()}
				value={store.query}
				oninput={handleInput}
				onkeydown={handleKeydown}
				aria-controls="search-result-rows"
				aria-activedescendant={selectedIndex >= 0 ? `search-result-${selectedIndex}` : undefined}
				data-testid="global-search-input"
			>
				{#snippet leading()}
					<SearchIcon class="h-5 w-5" />
				{/snippet}
			</TextField>
		</div>
		<ClusterSelector
			options={store.clusterOptions}
			value={store.cluster}
			onChange={(value: string) => store.setCluster(value)}
			id="search-cluster-filter"
			includeAll
		/>
		<label for="search-status-filter" class="sr-only">{m['vms.list.filterStatusLabel']()}</label>
		<Select
			id="search-status-filter"
			value={store.status}
			options={statusOptions}
			onchange={handleStatusChange}
			class="w-auto min-w-[9rem]"
			data-testid="search-status-filter"
		/>
	</div>
	<p class="mb-6 text-xs text-muted-foreground-subtle">{m['search.hint']()}</p>

	{#if store.loading && store.result === null}
		<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
		<TableSkeleton columns={6} />
	{:else if store.error}
		<Alert data-testid="search-error">{store.error}</Alert>
	{:else if store.result === null}
		<!-- Idle: the hint line above is the whole state; no card, no filler. -->
	{:else if store.result.items.length === 0}
		<TableCard>
			<EmptyState title={m['search.empty']()} dataTestid="search-empty" />
		</TableCard>
	{:else}
		<TableCard>
			<table class="pv-table pv-responsive-table" id="search-result-rows">
				<caption class="sr-only">{m['search.resultsCaption']()}</caption>
				<thead>
					<tr>
						<TableHeader
							text={m['vms.list.columnMachine']()}
							column="name"
							activeColumn={store.sortBy}
							sortDir={store.sortDir}
							onSort={(column) => store.setSort(column)}
						/>
						<TableHeader
							text={m['vms.list.columnId']()}
							column="vmid"
							activeColumn={store.sortBy}
							sortDir={store.sortDir}
							onSort={(column) => store.setSort(column)}
						/>
						<TableHeader text={m['vms.list.columnNode']()} />
						{#if spansClusters}
							<TableHeader text={m['vms.list.columnCluster']()} />
						{/if}
						<TableHeader text={m['vms.list.columnResources']()} />
						<TableHeader text={m['vms.list.columnStatus']()} />
						<TableHeader text={m['vms.list.columnActions']()} class="text-right" />
					</tr>
				</thead>
				<tbody>
					{#each items as machine, index (`${machine.cluster}:${machine.vmid}`)}
						{@const status = statusOf(machine)}
						{@const inFlight = busy(machine)}
						<tr
							id="search-result-{index}"
							class="relative"
							data-testid="search-result-row"
							data-selected={index === selectedIndex}
							aria-selected={index === selectedIndex}
							onmouseenter={() => (selectedIndex = index)}
						>
							<td data-label={m['vms.list.columnMachine']()}>
								<div class="flex min-w-0 items-center gap-3">
									<span class="max-[369px]:hidden"><OsMark ostype={machine.ostype} /></span>
									<div class="min-w-0">
										<a
											href={vmHref(machine)}
											class="pv-focus block max-w-full truncate font-medium text-foreground underline-offset-2 after:absolute after:inset-0 after:content-[''] hover:text-primary hover:underline"
											aria-label={spansClusters
												? m['vms.list.rowLinkLabel']({ name: machine.name, cluster: machine.clusterDisplayName })
												: m['vms.list.actionDetailsFor']({ name: machine.name })}
											data-testid="search-result-link"
										>
											{#each highlightText(machine.name, matchQuery) as segment, i (i)}
												{#if segment.match}<mark>{segment.text}</mark>{:else}{segment.text}{/if}
											{/each}
										</a>
										{#if visibleTags(machine).length > 0}
											<p class="truncate font-mono text-xs tabular-nums text-muted-foreground-subtle">
												{#each highlightText(visibleTags(machine).join(' · '), matchQuery) as segment, i (i)}
													{#if segment.match}<mark>{segment.text}</mark>{:else}{segment.text}{/if}
												{/each}
											</p>
										{/if}
									</div>
								</div>
							</td>
							<td data-label={m['vms.list.columnId']()}>
								<span class="font-mono text-xs tabular-nums">
									{#if vmidMatches(machine.vmid, matchQuery)}<mark>{machine.vmid}</mark>{:else}{machine.vmid}{/if}
								</span>
							</td>
							<td data-label={m['vms.list.columnNode']()}>
								<span class="whitespace-nowrap font-mono text-xs">{machine.node}</span>
							</td>
							{#if spansClusters}
								<td data-label={m['vms.list.columnCluster']()}>
									<span class="whitespace-nowrap font-mono text-xs">{machine.clusterDisplayName}</span>
								</td>
							{/if}
							<td data-label={m['vms.list.columnResources']()}>
								<span class="whitespace-nowrap font-mono text-xs tabular-nums">
									{m['vms.list.resourcesCompute']({ cpu: machine.cpuCores, memory: compactBytes(machine.memoryTotal) })}
								</span>
							</td>
							<td data-label={m['vms.list.columnStatus']()}>
								<MachineStatusPill {status} pending={inFlight || status === 'provisioning'} />
							</td>
							<td data-label={m['vms.list.columnActions']()} data-nolabel="true" class="relative z-10">
								<div class="flex items-center justify-end gap-2 whitespace-nowrap">
									{#if machine.status === 'running' && !inFlight}
										<ButtonLink
											variant="secondary"
											size="sm"
											href={consoleHref(machine)}
											aria-label={m['vms.list.actionConsoleFor']({ name: machine.name })}
											data-testid="search-row-console"
										>
											{m['vms.list.actionConsole']()}
										</ButtonLink>
									{/if}
									{#if machine.status === 'stopped'}
										<Button
											variant="secondary"
											size="sm"
											loading={powerActions.get(machine.cluster, machine.vmid) === 'start'}
											disabled={inFlight}
											aria-label={m['vms.list.actionStartFor']({ name: machine.name })}
											onclick={() => request(machine, 'start')}
											data-testid="search-row-start"
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
											data-testid="search-row-stop"
										>
											{m['vms.action.stop']()}
										</Button>
										<Button
											variant="secondary"
											size="sm"
											disabled={inFlight}
											aria-label={m['admin.vms.actionResetFor']({ name: machine.name })}
											onclick={() => request(machine, 'reset')}
											data-testid="search-row-reset"
										>
											{m['vms.action.reset']()}
										</Button>
									{/if}
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#snippet footer()}
				<div class="flex items-center justify-between border-t border-border px-4 py-2.5">
					<p class="text-xs text-muted-foreground" role="status" aria-live="polite" data-testid="search-result-count">
						{#if store.loadingMore}
							{m['common.loading']()}
						{:else}
							{m['search.showing']({ shown: store.result?.items.length ?? 0, total: store.result?.total ?? 0 })}
						{/if}
					</p>
					<div bind:this={sentinel} aria-hidden="true"></div>
				</div>
			{/snippet}
		</TableCard>
	{/if}
</section>

<ConfirmDialog
	open={pending !== null}
	title={confirmTitle}
	message={confirmMessage}
	confirmLabel={confirmLabel}
	cancelLabel={m['common.cancel']()}
	onConfirm={confirmPending}
	onClose={() => (pending = null)}
/>

<style>
	/* The rail reuses the hover accent so a keyboard-selected row reads the
	   same way a hovered one does, just persistent. */
	tr[data-selected='true'] {
		background: var(--muted);
	}
	tr[data-selected='true'] td:first-child {
		box-shadow: inset 2px 0 0 var(--primary);
	}
	mark {
		border-radius: 2px;
		background: color-mix(in oklab, var(--primary) 15%, transparent);
		color: inherit;
		padding: 0 1px;
	}
</style>
