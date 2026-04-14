<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { t } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import { getVMs, type VMSummary } from '$lib/api/vms';
	import { api } from '$lib/api/client';
	import {
		ArrowsClockwise, PlusSquare, Desktop, Play, Stop, ArrowCounterClockwise,
		CaretUp, CaretDown, ArrowsDownUp, CheckSquare, Square, ClockCounterClockwise
	} from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';

	// ── Types ──────────────────────────────────────────────────────────────
	type SortKey = 'vmid' | 'name' | 'status';
	type SortDir = 'asc' | 'desc';

	interface ActivityEntry {
		readonly id: string;
		readonly action: string;
		readonly vmName: string;
		readonly timestamp: Date;
	}

	// ── State ──────────────────────────────────────────────────────────────
	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let vms = $state<VMSummary[]>([]);
	let actionLoading = $state<Record<number, boolean>>({});
	let bulkLoading = $state(false);

	let sortKey = $state<SortKey>('vmid');
	let sortDir = $state<SortDir>('asc');
	let selected = $state<Set<number>>(new Set());
	let page = $state(1);
	let activityLog = $state<ActivityEntry[]>([]);

	const PER_PAGE = 10;

	// ── Derived ────────────────────────────────────────────────────────────
	const sortedVms = $derived(sortVms(vms, sortKey, sortDir));
	const totalPages = $derived(Math.max(1, Math.ceil(sortedVms.length / PER_PAGE)));
	const paginatedVms = $derived(sortedVms.slice((page - 1) * PER_PAGE, page * PER_PAGE));
	const stats = $derived(computeStats(vms));
	const allPageSelected = $derived(
		paginatedVms.length > 0 && paginatedVms.every((v) => selected.has(v.vmid))
	);
	const someSelected = $derived(selected.size > 0);

	// ── Pure helpers ───────────────────────────────────────────────────────
	function sortVms(list: VMSummary[], key: SortKey, dir: SortDir): VMSummary[] {
		return [...list].sort((a, b) => {
			let cmp = 0;
			if (key === 'vmid') cmp = a.vmid - b.vmid;
			else if (key === 'name') cmp = (a.name || '').localeCompare(b.name || '');
			else if (key === 'status') cmp = a.status.localeCompare(b.status);
			return dir === 'asc' ? cmp : -cmp;
		});
	}

	function computeStats(list: VMSummary[]) {
		const running = list.filter((v) => v.status === 'running');
		const avgCpu =
			running.length > 0
				? Math.round(running.reduce((s, v) => s + v.cpu * 100, 0) / running.length)
				: 0;
		return { total: list.length, running: running.length, stopped: list.filter((v) => v.status === 'stopped').length, avgCpu };
	}

	function statusClass(status: string): string {
		if (status === 'running') return 'pv-badge--online';
		if (status === 'stopped') return 'pv-badge--offline';
		return 'pv-badge--warn';
	}

	function uptimeLabel(seconds: number): string {
		if (!seconds) return '—';
		const d = Math.floor(seconds / 86400);
		const h = Math.floor((seconds % 86400) / 3600);
		if (d > 0) return `${d}d ${h}h`;
		const m = Math.floor((seconds % 3600) / 60);
		return h > 0 ? `${h}h ${m}m` : `${m}m`;
	}

	function actionLabel(action: string): string {
		const labels: Record<string, string> = { start: 'Started', shutdown: 'Shutdown', stop: 'Stopped', reboot: 'Rebooted' };
		return labels[action] ?? action;
	}

	function timeAgo(date: Date): string {
		const s = Math.floor((Date.now() - date.getTime()) / 1000);
		if (s < 60) return 'just now';
		if (s < 3600) return `${Math.floor(s / 60)}m ago`;
		return `${Math.floor(s / 3600)}h ago`;
	}

	// ── Actions ────────────────────────────────────────────────────────────
	async function load(isRefresh = false): Promise<void> {
		if (isRefresh) refreshing = true;
		else loading = true;
		error = null;
		try {
			vms = await getVMs();
			if (page > Math.max(1, Math.ceil(vms.length / PER_PAGE))) page = 1;
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	async function doAction(vm: VMSummary, action: string): Promise<void> {
		actionLoading = { ...actionLoading, [vm.vmid]: true };
		try {
			await api.post(`/api/v1/vms/${vm.vmid}/action`, { action, node: vm.node });
			toast.success(`${action} sent to ${vm.name || vm.vmid}`);
			addActivity(action, vm.name || String(vm.vmid));
			setTimeout(() => load(true), 2000);
		} catch {
			toast.error(`Failed to ${action} ${vm.name || vm.vmid}`);
		} finally {
			actionLoading = { ...actionLoading, [vm.vmid]: false };
		}
	}

	async function doBulkAction(action: string): Promise<void> {
		bulkLoading = true;
		const targets = vms.filter((v) => selected.has(v.vmid));
		const results = await Promise.allSettled(
			targets.map((vm) => api.post(`/api/v1/vms/${vm.vmid}/action`, { action, node: vm.node }))
		);
		const ok = results.filter((r) => r.status === 'fulfilled').length;
		const fail = results.length - ok;
		if (ok > 0) {
			toast.success(`${action} sent to ${ok} VM(s)`);
			targets.forEach((vm) => addActivity(action, vm.name || String(vm.vmid)));
		}
		if (fail > 0) toast.error(`Failed for ${fail} VM(s)`);
		selected = new Set();
		bulkLoading = false;
		setTimeout(() => load(true), 2000);
	}

	function toggleSort(key: SortKey): void {
		if (sortKey === key) sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		else { sortKey = key; sortDir = 'asc'; }
		page = 1;
	}

	function toggleSelect(vmid: number): void {
		const next = new Set(selected);
		if (next.has(vmid)) next.delete(vmid);
		else next.add(vmid);
		selected = next;
	}

	function toggleSelectAll(): void {
		const next = new Set(selected);
		if (allPageSelected) paginatedVms.forEach((v) => next.delete(v.vmid));
		else paginatedVms.forEach((v) => next.add(v.vmid));
		selected = next;
	}

	function addActivity(action: string, vmName: string): void {
		activityLog = [
			{ id: crypto.randomUUID(), action, vmName, timestamp: new Date() },
			...activityLog.slice(0, 19)
		];
	}

	onMount(() => load());
</script>

<svelte:head>
	<title>PVMSS — {$t('user.home.title')}</title>
</svelte:head>

{#snippet sortIcon(key: SortKey)}
	{#if sortKey === key}
		{#if sortDir === 'asc'}
			<CaretUp class="h-3 w-3" weight="bold" />
		{:else}
			<CaretDown class="h-3 w-3" weight="bold" />
		{/if}
	{:else}
		<ArrowsDownUp class="h-3 w-3 opacity-25" />
	{/if}
{/snippet}

<div class="mx-auto px-4 py-6 pv-content-width">
	<!-- Header -->
	<div class="mb-5 flex items-start justify-between">
		<div>
			<h1 class="text-2xl font-bold">{$t('user.home.title')}</h1>
			{#if !loading && vms.length > 0}
				<p class="mt-0.5 text-sm text-muted-foreground">
					{vms.length}
					{vms.length === 1 ? 'virtual machine' : 'virtual machines'}
				</p>
			{/if}
		</div>
		<div class="flex items-center gap-2">
			<Button href="/vm/create" size="sm">
				<PlusSquare class="mr-1.5 h-4 w-4" />
				{$t('nav.createVm')}
			</Button>
			<Button variant="outline" size="sm" onclick={() => load(true)} disabled={refreshing}>
				<ArrowsClockwise class="h-4 w-4 {refreshing ? 'animate-spin' : ''}" />
			</Button>
		</div>
	</div>

	{#if error}
		<ErrorBanner {error} onRetry={() => load()} />
	{:else if loading}
		<LoadingSkeleton variant="card" rows={4} />
	{:else if vms.length === 0}
		<!-- Better empty state -->
		<div class="pv-empty-state">
			<div class="pv-empty-icon">
				<Desktop class="h-14 w-14" />
			</div>
			<h2 class="pv-empty-title">{$t('user.home.emptyTitle')}</h2>
			<p class="pv-empty-desc">{$t('user.home.emptyDesc')}</p>
			<Button href="/vm/create" size="lg">
				<PlusSquare class="mr-2 h-4 w-4" />
				{$t('user.home.emptyAction')}
			</Button>
		</div>
	{:else}
		<!-- Stats row -->
		<div class="mb-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
			<div class="pv-stat-card">
				<span class="pv-stat-label">{$t('user.home.stats.total')}</span>
				<span class="pv-stat-value">{stats.total}</span>
			</div>
			<div class="pv-stat-card pv-stat-card--running">
				<span class="pv-stat-label">{$t('user.home.stats.running')}</span>
				<span class="pv-stat-value">{stats.running}</span>
			</div>
			<div class="pv-stat-card">
				<span class="pv-stat-label">{$t('user.home.stats.stopped')}</span>
				<span class="pv-stat-value">{stats.stopped}</span>
			</div>
			<div class="pv-stat-card pv-stat-card--cpu">
				<span class="pv-stat-label">{$t('user.home.stats.avgCpu')}</span>
				<span class="pv-stat-value">{stats.avgCpu}%</span>
			</div>
		</div>

		<!-- Content grid: table + activity -->
		<div class="grid grid-cols-1 gap-4 lg:grid-cols-[1fr_264px]">
			<div class="min-w-0">
				<!-- Bulk action toolbar -->
				{#if someSelected}
					<div class="pv-bulk-toolbar mb-2">
						<span class="text-sm text-muted-foreground">
							{$t('vms.selected', { values: { count: selected.size } })}
						</span>
						<div class="flex gap-1.5">
							<Button size="sm" variant="outline" onclick={() => doBulkAction('start')} disabled={bulkLoading}>
								<Play class="mr-1 h-3.5 w-3.5" weight="fill" />
								{$t('vms.actions.start')}
							</Button>
							<Button size="sm" variant="outline" onclick={() => doBulkAction('shutdown')} disabled={bulkLoading}>
								<Stop class="mr-1 h-3.5 w-3.5" weight="fill" />
								{$t('vms.actions.shutdown')}
							</Button>
							<Button size="sm" variant="outline" onclick={() => doBulkAction('reboot')} disabled={bulkLoading}>
								<ArrowCounterClockwise class="mr-1 h-3.5 w-3.5" />
								{$t('vms.actions.reboot')}
							</Button>
							<Button size="sm" variant="ghost" onclick={() => (selected = new Set())}>
								{$t('vms.clearSelection')}
							</Button>
						</div>
					</div>
				{/if}

				<!-- Table -->
				<div class="pv-table-wrap">
					<table class="pv-table">
						<thead>
							<tr>
								<th class="w-10">
									<button class="pv-check-btn" onclick={toggleSelectAll} aria-label="Select all">
										{#if allPageSelected}
											<CheckSquare class="h-4 w-4 text-primary" weight="fill" />
										{:else}
											<Square class="h-4 w-4 text-muted-foreground" />
										{/if}
									</button>
								</th>
								<th>
									<button class="pv-sort-btn" onclick={() => toggleSort('vmid')}>
										{$t('vms.vmid')}
										{@render sortIcon('vmid')}
									</button>
								</th>
								<th>
									<button class="pv-sort-btn" onclick={() => toggleSort('name')}>
										{$t('common.name')}
										{@render sortIcon('name')}
									</button>
								</th>
								<th>
									<button class="pv-sort-btn" onclick={() => toggleSort('status')}>
										{$t('common.status')}
										{@render sortIcon('status')}
									</button>
								</th>
								<th>{$t('vms.cpu')}</th>
								<th>{$t('vms.ram')}</th>
								<th>{$t('vms.uptime')}</th>
								<th></th>
							</tr>
						</thead>
						<tbody>
							{#each paginatedVms as vm (vm.vmid)}
								{@const busy = actionLoading[vm.vmid] ?? false}
								{@const isSelected = selected.has(vm.vmid)}
								<tr
									class="pv-row pv-row--clickable {isSelected ? 'pv-row--selected' : ''}"
									onclick={() => goto(`/vm/${vm.vmid}`)}
								>
									<td
										class="w-10"
										onclick={(e) => {
											e.stopPropagation();
											toggleSelect(vm.vmid);
										}}
									>
										{#if isSelected}
											<CheckSquare class="h-4 w-4 text-primary" weight="fill" />
										{:else}
											<Square class="h-4 w-4 text-muted-foreground opacity-40" />
										{/if}
									</td>
									<td class="pv-td-mono text-sm">{vm.vmid}</td>
									<td>
										<div class="pv-resource-cell">
											<div class="pv-resource-icon pv-resource-icon--vm text-[0.6rem]">VM</div>
											<span class="pv-resource-name">{vm.name || '—'}</span>
										</div>
									</td>
									<td>
										<span class="pv-badge {statusClass(vm.status)}">
											{$t(`common.statusMap.${vm.status}`, { default: vm.status })}
										</span>
									</td>
									<td class="tabular-nums text-sm">
										{#if vm.status === 'running'}
											<div class="pv-usage-bar w-24">
												<div class="pv-usage-bar-track" style="flex:1">
													<div class="pv-usage-bar-fill" style="width:{Math.round(vm.cpu * 100)}%"></div>
												</div>
												<span class="pv-usage-label">{Math.round(vm.cpu * 100)}%</span>
											</div>
										{:else}
											<span class="text-muted-foreground">—</span>
										{/if}
									</td>
									<td class="tabular-nums text-sm">
										{#if vm.max_mem_mb > 0}
											<div class="pv-usage-bar w-24">
												<div class="pv-usage-bar-track" style="flex:1">
													<div class="pv-usage-bar-fill" style="width:{Math.round((vm.mem_mb / vm.max_mem_mb) * 100)}%"></div>
												</div>
												<span class="pv-usage-label">{Math.round(vm.max_mem_mb / 1024)} GB</span>
											</div>
										{:else}
											<span class="text-muted-foreground">—</span>
										{/if}
									</td>
									<td class="pv-td-muted tabular-nums text-sm">{uptimeLabel(vm.uptime)}</td>
									<td onclick={(e) => e.stopPropagation()}>
										<div class="flex items-center gap-1">
											{#if vm.status === 'stopped'}
												<button
													class="pv-action-btn pv-action-btn--start"
													onclick={() => doAction(vm, 'start')}
													disabled={busy}
													title={$t('vms.actions.start')}
												>
													<Play class="h-3.5 w-3.5" weight="fill" />
												</button>
											{:else if vm.status === 'running'}
												<button
													class="pv-action-btn pv-action-btn--stop"
													onclick={() => doAction(vm, 'shutdown')}
													disabled={busy}
													title={$t('vms.actions.shutdown')}
												>
													<Stop class="h-3.5 w-3.5" weight="fill" />
												</button>
												<button
													class="pv-action-btn"
													onclick={() => doAction(vm, 'reboot')}
													disabled={busy}
													title={$t('vms.actions.reboot')}
												>
													<ArrowCounterClockwise class="h-3.5 w-3.5" />
												</button>
											{/if}
										</div>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>

				<!-- Pagination -->
				{#if totalPages > 1}
					<div class="mt-3 flex items-center justify-between">
						<span class="text-xs text-muted-foreground">
							{$t('common.pageOf', { values: { page, total: totalPages } })}
						</span>
						<div class="flex gap-1.5">
							<Button
								size="sm"
								variant="outline"
								onclick={() => (page -= 1)}
								disabled={page <= 1}
							>
								{$t('common.previous')}
							</Button>
							<Button
								size="sm"
								variant="outline"
								onclick={() => (page += 1)}
								disabled={page >= totalPages}
							>
								{$t('common.next')}
							</Button>
						</div>
					</div>
				{/if}
			</div>

			<!-- Activity feed -->
			<div class="pv-activity-panel">
				<div class="pv-activity-header">
					<ClockCounterClockwise class="h-4 w-4 shrink-0" />
					<span>{$t('user.home.activity')}</span>
				</div>
				{#if activityLog.length === 0}
					<p class="pv-activity-empty">{$t('user.home.noActivity')}</p>
				{:else}
					<ul class="pv-activity-list">
						{#each activityLog as entry (entry.id)}
							<li class="pv-activity-item">
								<div class="pv-activity-dot"></div>
								<div class="pv-activity-body">
									<span class="pv-activity-action">{actionLabel(entry.action)}</span>
									<span class="pv-activity-vm">{entry.vmName}</span>
									<span class="pv-activity-time">{timeAgo(entry.timestamp)}</span>
								</div>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
		</div>
	{/if}
</div>

<style>
	/* ── Clickable rows ─────────────────────────────────────────────── */
	:global(.pv-row--clickable) {
		cursor: pointer;
	}
	:global(.pv-row--clickable:hover td) {
		background: var(--accent);
	}
	:global(.pv-row--selected td) {
		background: hsl(var(--primary) / 0.06);
	}
	:global(.pv-row--selected:hover td) {
		background: hsl(var(--primary) / 0.1);
	}

	/* ── Action buttons ─────────────────────────────────────────────── */
	:global(.pv-action-btn) {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border-radius: 6px;
		border: 1px solid var(--border);
		background: transparent;
		color: var(--muted-foreground);
		cursor: pointer;
		transition: background 0.12s, color 0.12s, border-color 0.12s;
	}
	:global(.pv-action-btn:disabled) {
		opacity: 0.4;
		cursor: not-allowed;
	}
	:global(.pv-action-btn:hover:not(:disabled)) {
		background: var(--accent);
		color: var(--accent-foreground);
	}
	:global(.pv-action-btn--start:hover:not(:disabled)) {
		background: hsl(142 71% 45% / 0.15);
		border-color: hsl(142 71% 45% / 0.4);
		color: hsl(142 71% 35%);
	}
	:global(.pv-action-btn--stop:hover:not(:disabled)) {
		background: hsl(0 84% 60% / 0.15);
		border-color: hsl(0 84% 60% / 0.4);
		color: hsl(0 84% 50%);
	}

	/* ── Checkbox button ────────────────────────────────────────────── */
	:global(.pv-check-btn) {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		background: none;
		border: none;
		cursor: pointer;
		padding: 2px;
		border-radius: 4px;
	}
	:global(.pv-check-btn:hover) {
		background: var(--accent);
	}

	/* ── Sortable header buttons ────────────────────────────────────── */
	:global(.pv-sort-btn) {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		background: none;
		border: none;
		cursor: pointer;
		font-size: inherit;
		font-weight: inherit;
		color: inherit;
		padding: 0;
		white-space: nowrap;
	}
	:global(.pv-sort-btn:hover) {
		color: var(--foreground);
	}

	/* ── Stats cards ────────────────────────────────────────────────── */
	:global(.pv-stat-card) {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding: 12px 16px;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--card);
	}
	:global(.pv-stat-label) {
		font-size: 0.75rem;
		color: var(--muted-foreground);
		font-weight: 500;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}
	:global(.pv-stat-value) {
		font-size: 1.5rem;
		font-weight: 700;
		line-height: 1;
		color: var(--foreground);
		font-variant-numeric: tabular-nums;
	}
	:global(.pv-stat-card--running .pv-stat-value) {
		color: hsl(142 71% 35%);
	}
	:global(.pv-stat-card--cpu .pv-stat-value) {
		color: hsl(221 83% 53%);
	}

	/* ── Bulk toolbar ───────────────────────────────────────────────── */
	:global(.pv-bulk-toolbar) {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 8px 12px;
		background: var(--accent);
		border: 1px solid var(--border);
		border-radius: 8px;
		flex-wrap: wrap;
		gap: 8px;
	}

	/* ── Empty state ────────────────────────────────────────────────── */
	:global(.pv-empty-state) {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 12px;
		padding: 80px 24px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--card);
		text-align: center;
	}
	:global(.pv-empty-icon) {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 72px;
		height: 72px;
		border-radius: 50%;
		background: var(--muted);
		color: var(--muted-foreground);
		opacity: 0.6;
	}
	:global(.pv-empty-title) {
		font-size: 1.125rem;
		font-weight: 600;
		color: var(--foreground);
	}
	:global(.pv-empty-desc) {
		font-size: 0.875rem;
		color: var(--muted-foreground);
		max-width: 320px;
	}

	/* ── Activity panel ─────────────────────────────────────────────── */
	:global(.pv-activity-panel) {
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--card);
		overflow: hidden;
		height: fit-content;
	}
	:global(.pv-activity-header) {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 12px 14px;
		border-bottom: 1px solid var(--border);
		font-size: 0.8125rem;
		font-weight: 600;
		color: var(--foreground);
	}
	:global(.pv-activity-empty) {
		padding: 24px 14px;
		font-size: 0.8125rem;
		color: var(--muted-foreground);
		text-align: center;
	}
	:global(.pv-activity-list) {
		list-style: none;
		margin: 0;
		padding: 8px 0;
	}
	:global(.pv-activity-item) {
		display: flex;
		align-items: flex-start;
		gap: 10px;
		padding: 7px 14px;
	}
	:global(.pv-activity-dot) {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--primary);
		margin-top: 5px;
		flex-shrink: 0;
		opacity: 0.7;
	}
	:global(.pv-activity-body) {
		display: flex;
		flex-direction: column;
		gap: 1px;
		min-width: 0;
	}
	:global(.pv-activity-action) {
		font-size: 0.75rem;
		font-weight: 600;
		color: var(--foreground);
	}
	:global(.pv-activity-vm) {
		font-size: 0.75rem;
		color: var(--muted-foreground);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	:global(.pv-activity-time) {
		font-size: 0.6875rem;
		color: var(--muted-foreground);
		opacity: 0.7;
	}
</style>
