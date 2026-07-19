<script lang="ts">
	import { untrack } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { goto } from '$app/navigation';
	import { t } from 'svelte-i18n';
	import { get } from 'svelte/store';
	import { flip } from 'svelte/animate';
	import { fade } from 'svelte/transition';
	import { Button } from '$lib/components/ui/button';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import VMUsageBar from '$lib/components/data/VMUsageBar.svelte';
	import { getVMsPaginated, type VMSummary } from '$lib/api/vms';
	import type { VMStatus } from '$lib/types/vm';
	import { vmList } from '$lib/utils/vm';
	import { api } from '$lib/api/client';
	import { auth } from '$lib/stores/auth.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import {
		ArrowsClockwise, PlusSquare, Desktop, Play, Stop, ArrowCounterClockwise,
		CaretUp, CaretDown, ArrowsDownUp, CheckSquare, Square, ClockCounterClockwise,
		MagnifyingGlass, X, Monitor
	} from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';

	// ── Types ──────────────────────────────────────────────────────────────
	interface ActivityEntry {
		readonly id: string;
		readonly action: string;
		readonly vmName: string;
		readonly timestamp: Date;
	}

	// ── Constants ──────────────────────────────────────────────────────────
	const DEFAULT_PAGE_SIZE = 10;
	const AUTO_REFRESH_INTERVAL_MS = 30_000;

	const STATUS_FILTERS: readonly ('' | VMStatus)[] = ['', 'running', 'stopped', 'paused'];

	// ── State ──────────────────────────────────────────────────────────────
	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let vms = $state<VMSummary[]>([]);
	let actionLoading = $state<Record<number, boolean>>({});
	let bulkLoading = $state(false);

	const selected = new SvelteSet<number>();
	let activityLog = $state<ActivityEntry[]>([]);
	let activityCollapsed = $state(false);
	let nextRefreshIn = $state(AUTO_REFRESH_INTERVAL_MS / 1000);

	// Server-driven pagination / search / sort (replaces client-side filter+slice)
	let currentPage = $state(1);
	let pageSize = $state(DEFAULT_PAGE_SIZE);
	let totalVMs = $state(0);
	let totalPages = $state(1);
	let hasNext = $state(false);
	let hasPrev = $state(false);
	let searchQuery = $state('');
	let sortBy = $state<string>('vmid');
	let sortOrder = $state<string>('asc');

	// Stage 4: server-side status + node filters (passed to the paginated endpoint)
	let filterStatus = $state<'' | 'running' | 'stopped' | 'paused'>('');
	let filterNode = $state('');
	let knownNodes = $state<string[]>([]);

	// Quota state (non-admin users only) – always reflects full owned count
	let maxVmPerUser = $state(0);
	let userTotalVMs = $state(0);

	// Server pagination aggregates for accurate stat cards (total/running/stopped across all pages for the current filter)
	let runningTotal = $state(0);
	let stoppedTotal = $state(0);

	// Abort + debounce helpers for loads
	let loadAbort: AbortController | null = null;
	let searchTimeout: ReturnType<typeof setTimeout> | null = null;

	// ── Derived (server-driven) ────────────────────────────────────────────
	const stats = $derived(computeStats(vms));
	const allPageSelected = $derived(
		vms.length > 0 && vms.every((v) => selected.has(v.vmid))
	);
	const someSelected = $derived(selected.size > 0);

	// For bulk action UX: know how many of the current-page selection are running vs stopped.
	// This lets us show only relevant bulk buttons and give better feedback.
	const selectedStopped = $derived(vms.filter((v) => selected.has(v.vmid) && v.status === 'stopped'));
	const selectedRunning = $derived(vms.filter((v) => selected.has(v.vmid) && v.status === 'running'));

	// Quota uses the full owned VM count (from pagination.total for non-admins)
	const quotaUsed = $derived(maxVmPerUser > 0 ? userTotalVMs : 0);
	const quotaPct = $derived(
		maxVmPerUser > 0 ? Math.round((quotaUsed / maxVmPerUser) * 100) : 0
	);

	// ── Pure helpers ───────────────────────────────────────────────────────
	// computeStats works on the current page slice (server already applied filters/sort).
	// running/stopped counts in the stats header come from server pagination metadata for accuracy.
	type SortColumn = 'vmid' | 'name' | 'status';

	function computeStats(list: VMSummary[]) {
		const running = list.filter((v) => v.status === 'running');
		const avgCpu =
			running.length > 0
				? Math.round(running.reduce((s, v) => s + v.cpu * 100, 0) / running.length)
				: 0;
		return { total: list.length, running: running.length, stopped: list.filter((v) => v.status === 'stopped').length, avgCpu };
	}

	function actionLabel(action: string): string {
		const translate = get(t) as (key: string, options?: Record<string, unknown>) => string;
		const keyMap: Record<string, string> = {
			start: 'user.home.activityLabels.start',
			shutdown: 'user.home.activityLabels.shutdown',
			stop: 'user.home.activityLabels.stop',
			reboot: 'user.home.activityLabels.reboot'
		};
		const key = keyMap[action];
		return key ? translate(key) : action;
	}

	function timeAgo(date: Date): string {
		const translate = get(t) as (key: string, options?: Record<string, unknown>) => string;
		const s = Math.floor((Date.now() - date.getTime()) / 1000);
		if (s < 60) return translate('common.time.justNow');
		if (s < 3600) return translate('common.time.mAgo', { values: { count: Math.floor(s / 60) } });
		return translate('common.time.hAgo', { values: { count: Math.floor(s / 3600) } });
	}

	// ── Auto-refresh ───────────────────────────────────────────────────────
	let autoRefreshTimer: ReturnType<typeof setInterval> | null = null;
	let countdownTimer: ReturnType<typeof setInterval> | null = null;

	function startAutoRefresh(): void {
		stopAutoRefresh();
		nextRefreshIn = AUTO_REFRESH_INTERVAL_MS / 1000;
		autoRefreshTimer = setInterval(() => {
			void loadVMs(true);
			nextRefreshIn = AUTO_REFRESH_INTERVAL_MS / 1000;
		}, AUTO_REFRESH_INTERVAL_MS);
		countdownTimer = setInterval(() => {
			nextRefreshIn = Math.max(0, nextRefreshIn - 1);
		}, 1000);
	}

	function stopAutoRefresh(): void {
		if (autoRefreshTimer !== null) { clearInterval(autoRefreshTimer); autoRefreshTimer = null; }
		if (countdownTimer !== null) { clearInterval(countdownTimer); countdownTimer = null; }
	}

	// ── Data loading (server pagination / search / sort) ───────────────────
	async function loadVMs(isRefresh = false): Promise<void> {
		if (loadAbort) loadAbort.abort();
		const abort = new AbortController();
		loadAbort = abort;

		if (isRefresh) refreshing = true;
		else loading = true;
		error = null;

		try {
			const res = await getVMsPaginated({
				page: currentPage,
				limit: pageSize,
				sortBy: sortBy,
				sortOrder: sortOrder,
				...(searchQuery && { search: searchQuery }),
				...(filterStatus && { status: filterStatus }),
				...(filterNode && { node: filterNode }),
			});
			if (abort.signal.aborted) return;

			vms = res.vms;
			totalVMs = res.pagination.total;
			totalPages = res.pagination.totalPages;
			hasNext = res.pagination.hasNext;
			hasPrev = res.pagination.hasPrev;
			runningTotal = res.pagination.runningCount ?? 0;
			stoppedTotal = res.pagination.stoppedCount ?? 0;

			// Stage 4: accumulate nodes seen so far for the node filter dropdown.
			// We union across loads so the dropdown stays useful even as the user pages/filters.
			const pageNodes = res.vms.map((v) => v.node).filter(Boolean);
			knownNodes = [...new Set([...knownNodes, ...pageNodes])].sort();

			// For non-admins the backend returns the user's full owned count via pool filtering.
			if (!auth.isAdmin) {
				userTotalVMs = totalVMs;
			}

			// Clamp page if it went out of range (e.g. after deletes or search)
			if (currentPage > totalPages) {
				currentPage = Math.max(1, totalPages);
			}

			// Drop any selections that are no longer visible on this page
			const visible = new Set(vms.map((v) => v.vmid));
			for (const id of [...selected]) {
				if (!visible.has(id)) selected.delete(id);
			}

			if (isRefresh) nextRefreshIn = AUTO_REFRESH_INTERVAL_MS / 1000;
		} catch (err: unknown) {
			if (abort.signal.aborted) return;
			error = err instanceof Error ? err : new Error(String(err));
		} finally {
			if (!abort.signal.aborted) {
				loading = false;
				refreshing = false;
			}
			if (loadAbort === abort) loadAbort = null;
		}
	}

	async function loadQuota(): Promise<void> {
		if (auth.isAdmin) return;
		try {
			const s = await settingsStore.fetchSettings();
			maxVmPerUser = s.maxVmPerUser;
		} catch {
			// quota display is non-critical
		}
	}

	// ── Search / sort / pagination controls ────────────────────────────────
	function onSearchInput(e: Event): void {
		if (searchTimeout) clearTimeout(searchTimeout);
		searchQuery = (e.target as HTMLInputElement).value;
		searchTimeout = setTimeout(() => {
			currentPage = 1;
			void loadVMs();
		}, 300);
	}

	function clearSearch(): void {
		searchQuery = '';
		currentPage = 1;
		void loadVMs();
	}

	function toggleSort(column: SortColumn): void {
		if (sortBy === column) {
			sortOrder = sortOrder === 'asc' ? 'desc' : 'asc';
		} else {
			sortBy = column;
			sortOrder = 'asc';
		}
		currentPage = 1;
		void loadVMs();
	}

	function goToPage(page: number): void {
		if (page < 1 || page > totalPages) return;
		currentPage = page;
		void loadVMs();
	}

	// ── Stage 4: status / node filter handlers ─────────────────────────────
	function setFilterStatus(value: '' | VMStatus): void {
		if (filterStatus === value) return;
		filterStatus = value;
		currentPage = 1;
		void loadVMs();
	}

	function setFilterNode(value: string): void {
		if (filterNode === value) return;
		filterNode = value;
		currentPage = 1;
		void loadVMs();
	}

	function clearAllFilters(): void {
		searchQuery = '';
		filterStatus = '';
		filterNode = '';
		currentPage = 1;
		void loadVMs();
	}

	// ── Selection ──────────────────────────────────────────────────────────
	function toggleSelect(vmid: number): void {
		if (selected.has(vmid)) {
			selected.delete(vmid);
		} else {
			selected.add(vmid);
		}
	}

	function toggleSelectAll(): void {
		if (allPageSelected) {
			vms.forEach((v) => selected.delete(v.vmid));
		} else {
			vms.forEach((v) => selected.add(v.vmid));
		}
	}

	// ── Actions ────────────────────────────────────────────────────────────
	async function doAction(vm: VMSummary, action: string): Promise<void> {
		actionLoading = { ...actionLoading, [vm.vmid]: true };
		const name = vm.name || String(vm.vmid);
		try {
			await api.post(`/api/v1/vms/${vm.vmid}/action`, { action, node: vm.node });

			// Optimistic feedback: flip status immediately for snappy UI.
			// Background refresh will reconcile with real server state.
			const target = vms.find((v) => v.vmid === vm.vmid);
			if (target) {
				const next: VMStatus = (action === 'start' || action === 'reboot') ? 'running' : 'stopped';
				if (target.status !== next) {
					// direct mutation on $state object is reactive
					target.status = next;
				}
			}

			toast.success($t('user.home.toast.actionSent', { values: { action, name } }));
			addActivity(action, name);
			setTimeout(() => loadVMs(true), 2000);
		} catch (err: unknown) {
			console.error(`VM action ${action} failed:`, err instanceof Error ? err.message : String(err));
			toast.error($t('user.home.toast.actionFailed', { values: { action, name } }));
		} finally {
			actionLoading = { ...actionLoading, [vm.vmid]: false };
		}
	}

	async function doBulkAction(action: string): Promise<void> {
		bulkLoading = true;

		// Stage 3 UX: only target VMs for which the action makes sense on the current page selection.
		let targets: VMSummary[];
		if (action === 'start') {
			targets = vms.filter((v) => selected.has(v.vmid) && v.status === 'stopped');
		} else if (action === 'shutdown' || action === 'reboot') {
			targets = vms.filter((v) => selected.has(v.vmid) && v.status === 'running');
		} else {
			targets = vms.filter((v) => selected.has(v.vmid));
		}

		if (targets.length === 0) {
			bulkLoading = false;
			return;
		}

		const results = await Promise.allSettled(
			targets.map((vm) => api.post(`/api/v1/vms/${vm.vmid}/action`, { action, node: vm.node }))
		);
		const ok = results.filter((r) => r.status === 'fulfilled').length;
		const fail = results.length - ok;

		if (ok > 0) {
			toast.success($t('user.home.toast.bulkActionSent', { values: { action, count: ok } }));
			targets.forEach((vm) => addActivity(action, vm.name || String(vm.vmid)));
		}
		if (fail > 0) toast.error($t('user.home.toast.bulkActionFailed', { values: { count: fail } }));

		// Clear only the VMs we actually acted on (keep other selections if mixed state).
		const acted = new Set(targets.map((v) => v.vmid));
		for (const id of [...selected]) {
			if (acted.has(id)) selected.delete(id);
		}

		bulkLoading = false;
		setTimeout(() => loadVMs(true), 2000);
	}

	function openConsole(vm: VMSummary): void {
		if (vm.status !== 'running') return;
		const url = `/vm/${vm.vmid}/console`;
		window.open(url, '_blank', 'width=1024,height=768,resizable=yes,scrollbars=yes');
	}

	function manualRefresh(): void {
		void loadVMs(true);
		startAutoRefresh();
	}

	function addActivity(action: string, vmName: string): void {
		activityLog = [
			{ id: crypto.randomUUID(), action, vmName, timestamp: new Date() },
			...activityLog.slice(0, 19)
		];
	}

	function clearActivity(): void {
		activityLog = [];
	}

	function toggleActivity(): void {
		activityCollapsed = !activityCollapsed;
	}

	function onRowKeydown(e: KeyboardEvent, vm: VMSummary): void {
		if (e.key === 'Enter') {
			e.preventDefault();
			goto(`/vm/${vm.vmid}`);
		} else if (e.key === ' ') {
			e.preventDefault();
			toggleSelect(vm.vmid);
		}
	}

	// Svelte 5 idiomatic lifecycle (Phase 1.3 of the modernisation plan).
	// $effect replaces onMount/onDestroy for side-effects that need cleanup.
	$effect(() => {
		untrack(() => {
			void loadVMs();
			void loadQuota();
			startAutoRefresh();
		});

		// Cleanup returned from the effect runs on component destroy or before re-running.
		return () => {
			stopAutoRefresh();
			if (searchTimeout) clearTimeout(searchTimeout);
			if (loadAbort) loadAbort.abort();
		};
	});
</script>

<svelte:head>
	<title>PVMSS — {$t('user.home.title')}</title>
</svelte:head>

{#snippet sortIcon(col: SortColumn)}
	{#if sortBy === col}
		{#if sortOrder === 'asc'}
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
			{#if !loading}
				<p class="mt-0.5 text-sm text-muted-foreground">
					{$t('user.home.vmCount', { values: { count: totalVMs } })}
				</p>
			{/if}
		</div>
		<div class="flex items-center gap-2">
			{#if !auth.isAdmin}
			<Button href="/vm/create" size="sm">
				<PlusSquare class="mr-1.5 h-4 w-4" />
				{$t('nav.createVm')}
			</Button>
		{/if}
			<!-- Auto-refresh indicator + manual refresh button -->
			<div class="flex items-center gap-1.5">
				{#if !loading && !refreshing}
					<span class="pv-refresh-countdown" title={$t('user.home.autoRefreshIn', { values: { seconds: nextRefreshIn } })}>{nextRefreshIn}s</span>
				{/if}
				<Button variant="outline" size="sm" onclick={manualRefresh} disabled={refreshing}>
					<ArrowsClockwise class="h-4 w-4 {refreshing ? 'animate-spin' : ''}" />
				</Button>
			</div>
		</div>
	</div>

	{#if error}
		<div transition:fade={{ duration: 120 }}>
			<ErrorBanner {error} onRetry={() => loadVMs()} />
		</div>
	{:else if loading}
		<div transition:fade={{ duration: 120 }}>
			<LoadingSkeleton variant="card" rows={4} />
		</div>
	{:else if totalVMs === 0}
		<!-- Better empty state (user truly owns zero VMs) -->
		<div class="pv-empty-state" transition:fade={{ duration: 120 }}>
			<div class="pv-empty-icon">
				<Desktop class="h-14 w-14" />
			</div>
			<h2 class="pv-empty-title">{$t('user.home.emptyTitle')}</h2>
			<p class="pv-empty-desc">{$t('user.home.emptyDesc')}</p>
			{#if !auth.isAdmin}
			<Button href="/vm/create" size="lg">
				<PlusSquare class="mr-2 h-4 w-4" />
				{$t('user.home.emptyAction')}
			</Button>
		{/if}
		</div>
	{:else}
		<!-- Main results block (filters + table + activity). Fade the loaded state in. -->
		<div transition:fade={{ duration: 120 }}>
			<!-- Stats row (authoritative fleet counts from server pagination metadata) -->
			<div class="mb-4 grid grid-cols-2 gap-3 {maxVmPerUser > 0 && !auth.isAdmin ? 'sm:grid-cols-5' : 'sm:grid-cols-4'}" transition:fade={{ duration: 120 }}>
			<div class="pv-stat-card">
				<span class="pv-stat-label">{$t('user.home.stats.total')}</span>
				<span class="pv-stat-value">{totalVMs}</span>
			</div>
			<div class="pv-stat-card pv-stat-card--running">
				<span class="pv-stat-label">{$t('user.home.stats.running')}</span>
				<span class="pv-stat-value">{runningTotal}</span>
			</div>
			<div class="pv-stat-card">
				<span class="pv-stat-label">{$t('user.home.stats.stopped')}</span>
				<span class="pv-stat-value">{stoppedTotal}</span>
			</div>
			<div class="pv-stat-card pv-stat-card--cpu">
				<span class="pv-stat-label">{$t('user.home.stats.avgCpu')}</span>
				<span class="pv-stat-value">{stats.avgCpu}%</span>
			</div>
			{#if maxVmPerUser > 0 && !auth.isAdmin}
				<div class="pv-stat-card pv-stat-card--quota">
					<span class="pv-stat-label">{$t('user.home.stats.quota')}</span>
					<span class="pv-stat-value">{quotaUsed}/{maxVmPerUser}</span>
					<div class="pv-quota-bar">
						<div
							class="pv-quota-bar-fill {quotaPct >= 90 ? 'pv-quota-bar-fill--danger' : quotaPct >= 70 ? 'pv-quota-bar-fill--warn' : ''}"
							style="width:{quotaPct}%"
						></div>
					</div>
				</div>
			{/if}
		</div>

		<!-- Inline search (server-driven) -->
		<div class="mb-3 relative">
			<MagnifyingGlass class="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground pointer-events-none" />
			<input
				type="text"
				class="pv-filter-input"
				placeholder={$t('user.home.filterPlaceholder')}
				bind:value={searchQuery}
				oninput={onSearchInput}
			/>
			{#if searchQuery}
				<button
					class="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
					onclick={clearSearch}
					aria-label={$t('common.clearSearch')}
				>
					<X class="h-3.5 w-3.5" />
				</button>
			{/if}
		</div>

		<!-- Stage 4: status + node filters (server-side, page resets on change) -->
		<div class="mb-3 flex flex-wrap items-center gap-2" transition:fade={{ duration: 120 }}>
			<select
				class="rounded-md border border-border bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
				value={filterStatus}
				onchange={(e: Event) => setFilterStatus((e.currentTarget as HTMLSelectElement).value as '' | VMStatus)}
			>
				<option value="">{$t('common.allStatuses')}</option>
				{#each STATUS_FILTERS as s (s)}
					{#if s}
						<option value={s}>{$t(`common.statusMap.${s}`, { default: s })}</option>
					{/if}
				{/each}
			</select>

			<select
				class="rounded-md border border-border bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
				value={filterNode}
				onchange={(e: Event) => setFilterNode((e.currentTarget as HTMLSelectElement).value)}
			>
				<option value="">{$t('common.allNodes')}</option>
				{#each knownNodes as n (n)}
					<option value={n}>{n}</option>
				{/each}
			</select>

			{#if searchQuery || filterStatus || filterNode}
				<button
					class="rounded-md border border-border bg-background px-3 py-1.5 text-sm hover:bg-accent"
					onclick={clearAllFilters}
				>
					{$t('common.clearFilters')}
				</button>
			{/if}
		</div>

		<!-- Content grid: table + activity -->
		<div class="grid grid-cols-1 gap-4 lg:grid-cols-[1fr_264px]">
			<div class="min-w-0">
				<!-- Bulk action toolbar (context-aware) -->
				{#if someSelected}
					<div class="pv-bulk-toolbar mb-2" transition:fade={{ duration: 120 }}>
						<span class="text-sm text-muted-foreground">
							{$t('vms.selected', { values: { count: selected.size } })}
							{#if selectedRunning.length > 0 || selectedStopped.length > 0}
								<span class="text-muted-foreground/70">
									{$t('user.home.selectionSummary', { values: { running: selectedRunning.length, stopped: selectedStopped.length } })}
								</span>
							{/if}
						</span>
						<div class="flex gap-1.5">
							{#if selectedStopped.length > 0}
								<Button size="sm" variant="outline" onclick={() => doBulkAction('start')} disabled={bulkLoading}>
									<Play class="mr-1 h-3.5 w-3.5" weight="fill" />
									{$t('vms.actions.start')}
								</Button>
							{/if}
							{#if selectedRunning.length > 0}
								<Button size="sm" variant="outline" onclick={() => doBulkAction('shutdown')} disabled={bulkLoading}>
									<Stop class="mr-1 h-3.5 w-3.5" weight="fill" />
									{$t('vms.actions.shutdown')}
								</Button>
								<Button size="sm" variant="outline" onclick={() => doBulkAction('reboot')} disabled={bulkLoading}>
									<ArrowCounterClockwise class="mr-1 h-3.5 w-3.5" />
									{$t('vms.actions.reboot')}
								</Button>
							{/if}
							<Button size="sm" variant="ghost" onclick={() => selected.clear()}>
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
									<button class="pv-check-btn" onclick={toggleSelectAll} aria-label={$t('common.selectAll')}>
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
								<th class="hidden md:table-cell">{$t('common.node')}</th>
								<th class="hidden lg:table-cell">{$t('common.tags', { default: 'Tags' })}</th>
								<th>
									<button class="pv-sort-btn" onclick={() => toggleSort('status')}>
										{$t('common.status')}
										{@render sortIcon('status')}
									</button>
								</th>
								<th>{$t('vms.cpu')}</th>
								<th>{$t('vms.ram')}</th>
								<th class="hidden lg:table-cell">{$t('vms.disk', { default: 'Disk' })}</th>
								<th class="hidden lg:table-cell">{$t('vms.uptime')}</th>
								<th></th>
							</tr>
						</thead>
						<tbody>
							{#each vms as vm (vm.vmid)}
								{@const busy = actionLoading[vm.vmid] ?? false}
								{@const isSelected = selected.has(vm.vmid)}
								<tr
									class="pv-row pv-row--clickable {isSelected ? 'pv-row--selected' : ''}"
									onclick={() => goto(`/vm/${vm.vmid}`)}
									tabindex="0"
									onkeydown={(e: KeyboardEvent) => onRowKeydown(e, vm)}
									animate:flip={{ duration: 180 }}
								>
									<td
										class="w-10"
										onclick={(e: MouseEvent) => {
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
									<td class="pv-td-muted text-sm hidden md:table-cell">{vm.node || '—'}</td>
									<td class="text-xs text-muted-foreground hidden lg:table-cell">
										{#if vm.tags}
											{(vm.tags || '').split(';').filter(Boolean).join(', ')}
										{:else}
											—
										{/if}
									</td>
									<td>
										<span class="pv-badge {vmList.statusClass(vm.status)}">
											{$t(`common.statusMap.${vm.status}`, { default: vm.status })}
										</span>
									</td>
									<td class="tabular-nums text-sm">
										<VMUsageBar
											current={vm.cpu * 100}
											max={100}
											label="{Math.round(vm.cpu * 100)}%"
											widthClass="w-24"
										/>
									</td>
									<td class="tabular-nums text-sm">
										<VMUsageBar
											current={vm.memMb}
											max={vm.maxMemMb}
											label="{Math.round(vm.maxMemMb / 1024)} GB"
											widthClass="w-24"
										/>
									</td>
									<td class="tabular-nums text-sm hidden lg:table-cell">
										<VMUsageBar
											current={vm.diskMb}
											max={vm.maxDiskMb ?? 0}
											label="{Math.round((vm.maxDiskMb ?? 0) / 1024)} GB"
											widthClass="w-24"
										/>
									</td>
									<td class="pv-td-muted tabular-nums text-sm hidden lg:table-cell">{vmList.uptimeLabel(vm.uptime)}</td>
									<td onclick={(e: MouseEvent) => e.stopPropagation()}>
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
												<!-- Console before stop/shutdown (per request) -->
												<button
													class="pv-action-btn"
													onclick={() => openConsole(vm)}
													title={$t('vms.actions.console', { default: 'Console' })}
													aria-label={$t('vms.actions.console', { default: 'Console' })}
												>
													<Monitor class="h-3.5 w-3.5" weight="fill" />
												</button>
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

				<!-- Pagination (server-driven) -->
				{#if totalPages > 1}
					<div class="mt-3 flex items-center justify-between">
						<span class="text-xs text-muted-foreground">
							{$t('common.pageOf', { values: { page: currentPage, total: totalPages } })}
						</span>
						<div class="flex gap-1.5">
							<Button
								size="sm"
								variant="outline"
								onclick={() => goToPage(currentPage - 1)}
								disabled={!hasPrev}
							>
								{$t('common.previous')}
							</Button>
							<Button
								size="sm"
								variant="outline"
								onclick={() => goToPage(currentPage + 1)}
								disabled={!hasNext}
							>
								{$t('common.next')}
							</Button>
						</div>
					</div>
				{/if}
			</div>

			<!-- Activity feed -->
			<div class="pv-activity-panel">
				<div
					class="pv-activity-header cursor-pointer select-none"
					onclick={toggleActivity}
					title={$t('user.home.toggleActivity')}
					role="button"
					tabindex="0"
					onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggleActivity(); } }}
				>
					<ClockCounterClockwise class="h-4 w-4 shrink-0" />
					<span>{$t('user.home.activity')}</span>
					{#if activityLog.length > 0}
						<button
							class="ml-auto text-[10px] text-muted-foreground hover:text-foreground"
							onclick={(e: MouseEvent) => { e.stopPropagation(); clearActivity(); }}
							title={$t('user.home.clearActivity')}
						>
							{$t('user.home.clearActivity')}
						</button>
					{/if}
				</div>
				{#if !activityCollapsed}
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
				{/if}
			</div>
		</div>
		</div>
	{/if}
</div>

<style>
	/* ── Auto-refresh countdown ─────────────────────────────────────── */
	:global(.pv-refresh-countdown) {
		font-size: 0.7rem;
		color: var(--muted-foreground);
		opacity: 0.6;
		min-width: 2rem;
		text-align: right;
		font-variant-numeric: tabular-nums;
	}

	/* ── Quota stat card ─────────────────────────────────────────────── */
	:global(.pv-stat-card--quota) {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	:global(.pv-quota-bar) {
		height: 3px;
		border-radius: 2px;
		background: var(--border);
		margin-top: 4px;
		overflow: hidden;
	}
	:global(.pv-quota-bar-fill) {
		height: 100%;
		border-radius: 2px;
		background: var(--primary);
		transition: width 0.3s ease;
	}
	:global(.pv-quota-bar-fill--warn) {
		background: var(--warning, oklch(75% 0.18 75));
	}
	:global(.pv-quota-bar-fill--danger) {
		background: var(--destructive);
	}

	/* ── Inline filter ───────────────────────────────────────────────── */
	:global(.pv-filter-input) {
		width: 100%;
		height: 2rem;
		padding: 0 2rem 0 2.25rem;
		font-size: 0.8125rem;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: var(--background);
		color: var(--foreground);
		outline: none;
		transition: border-color 0.15s;
	}
	:global(.pv-filter-input:focus) {
		border-color: var(--primary);
	}
	:global(.pv-filter-input::placeholder) {
		color: var(--muted-foreground);
		opacity: 0.6;
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

	/* ── Row affordance (keyboard + mouse) ──────────────────────────── */
	:global(.pv-row--clickable:focus-visible) {
		outline: 2px solid var(--ring, var(--primary));
		outline-offset: -2px;
		background: color-mix(in oklab, var(--primary) 6%, transparent);
	}
	:global(.pv-row--clickable:hover) {
		background: color-mix(in oklab, var(--primary) 4%, var(--card));
	}

	/* Stronger selected state for bulk UX (left accent + boosted tint) */
	:global(.pv-row--selected) {
		box-shadow: inset 3px 0 0 var(--primary);
	}
	:global(.pv-row--selected td) {
		background: color-mix(in oklab, var(--primary) 10%, var(--card));
	}
	:global(.pv-row--selected:hover td) {
		background: color-mix(in oklab, var(--primary) 14%, var(--card));
	}

	/* ── Action button micro-feedback ───────────────────────────────── */
	:global(.pv-action-btn:active) {
		transform: scale(0.94);
		transition: transform 60ms ease;
	}
</style>
