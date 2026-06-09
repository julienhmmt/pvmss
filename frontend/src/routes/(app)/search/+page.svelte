<script lang="ts">
	import { browser } from '$app/environment';
	import { goto, replaceState } from '$app/navigation';
	import { page } from '$app/state';
	import { flip } from 'svelte/animate';
	import { t } from 'svelte-i18n';
	import { untrack } from 'svelte';
	import { debounce, autofocus } from '$lib/actions';
	import { getVMsPaginated, type VMPaginationParams, type VMSummary, type SearchType } from '$lib/api/vms';
	import type { VMStatus } from '$lib/types/vm';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import { vmList } from '$lib/utils/vm';
	import {
		ArrowDown,
		ArrowUp,
		Desktop,
		MagnifyingGlass,
		SpinnerGap,
		X,
		Cpu,
		Memory,
		HardDrive,
		Clock
	} from 'phosphor-svelte';

	const PAGE_SIZE = 25;
	const DEBOUNCE_DELAY_MS = 350;
	const SLOW_LOADING_DELAY_MS = 2500;
	const STATUS_FILTERS: readonly VMStatus[] = ['running', 'stopped', 'paused'];

	type SortColumn = 'vmid' | 'name' | 'node' | 'status' | 'cpu' | 'memory';
	type SortDirection = 'asc' | 'desc';

	function detectSearchType(query: string): SearchType {
		const trimmed = query.trim();
		if (!trimmed) return 'name';
		if (trimmed.startsWith('tag:') || trimmed.startsWith('#')) return 'tag';
		if (/^\d+$/.test(trimmed)) return 'vmid';
		return 'name';
	}

	function extractSearchQuery(query: string): string {
		const trimmed = query.trim();
		if (trimmed.startsWith('tag:')) return trimmed.substring(4).trim();
		if (trimmed.startsWith('#')) return trimmed.substring(1).trim();
		return trimmed;
	}

	function isVMStatus(value: string): value is VMStatus {
		return STATUS_FILTERS.includes(value as VMStatus);
	}

	function isSortColumn(v: string): v is SortColumn {
		return ['vmid', 'name', 'node', 'status', 'cpu', 'memory'].includes(v);
	}

	function isSortDir(v: string): v is SortDirection {
		return v === 'asc' || v === 'desc';
	}

	function parsePage(p: string | null): number {
		const n = p ? parseInt(p, 10) : 1;
		return Number.isFinite(n) && n > 0 ? n : 1;
	}

	let q = $state(page.url.searchParams.get('q') ?? '');
	let filterStatus = $state<VMStatus | ''>(
		isVMStatus(page.url.searchParams.get('status') ?? '') ? (page.url.searchParams.get('status') as VMStatus) : ''
	);
	let filterNode = $state(page.url.searchParams.get('node') ?? '');
	let sortCol = $state<SortColumn>(
		isSortColumn(page.url.searchParams.get('sortBy') ?? '') ? (page.url.searchParams.get('sortBy') as SortColumn) : 'vmid'
	);
	let sortDir = $state<SortDirection>(
		isSortDir(page.url.searchParams.get('sortOrder') ?? '') ? (page.url.searchParams.get('sortOrder') as SortDirection) : 'asc'
	);
	let currentPage = $state(parsePage(page.url.searchParams.get('page')));

	let loading = $state(false);
	let searched = $state(false);
	let error = $state<Error | null>(null);
	let vms = $state<VMSummary[]>([]);
	let total = $state(0);
	let totalPages = $state(1);
	let hasNext = $state(false);
	let hasPrev = $state(false);
	let nodesFacet = $state<string[]>([]);

	let slowLoadingVisible = $state(false);
	let hasMounted = $state(false);
	let loadAbort: AbortController | null = null;
	let slowTimer: ReturnType<typeof setTimeout> | null = null;

	const extractedQuery = $derived(extractSearchQuery(q));
	const detectedType = $derived(detectSearchType(q));
	const shouldSearch = $derived(Boolean(extractedQuery || filterStatus || filterNode));

	// Accumulate nodes from facets across searches so the dropdown stays useful.
	const knownNodes = $derived.by(() => {
		const set = new Set<string>(nodesFacet);
		return Array.from(set).sort();
	});

	// Keep local pagination state in sync when URL changes externally (back/forward/share).
	// Use untrack when reading local state so this effect does not subscribe to q/filter/etc.
	// Subscribing caused the effect to re-run on every keystroke; the comparison against a
	// (transitional) urlQ would then assign back, clobbering the input bind:value and
	// preventing typing. Untrack ensures we only react to real external page changes.
	$effect(() => {
		const sp = page.url.searchParams;

		const urlQ = sp.get('q') ?? '';
		if (urlQ !== untrack(() => q)) q = urlQ;

		const st = sp.get('status') ?? '';
		const urlStatus = isVMStatus(st) ? (st as VMStatus) : '';
		if (urlStatus !== untrack(() => filterStatus)) filterStatus = urlStatus;

		const urlNode = sp.get('node') ?? '';
		if (urlNode !== untrack(() => filterNode)) filterNode = urlNode;

		const sb = sp.get('sortBy') ?? '';
		const urlSortCol = isSortColumn(sb) ? (sb as SortColumn) : 'vmid';
		if (urlSortCol !== untrack(() => sortCol)) sortCol = urlSortCol;

		const so = sp.get('sortOrder') ?? '';
		const urlSortDir = isSortDir(so) ? (so as SortDirection) : 'asc';
		if (urlSortDir !== untrack(() => sortDir)) sortDir = urlSortDir;

		const urlPage = parsePage(sp.get('page'));
		if (urlPage !== untrack(() => currentPage)) currentPage = urlPage;
	});

	// When filters or sort change (except page), reset to page 1 and push URL.
	// NOTE: q (search text) is deliberately omitted here. It is handled exclusively by the
	// debounced handler on the input (use:debounce) and explicit forceSearch/clear paths.
	// Reacting to q in this effect caused an immediate URL push on every keystroke, stealing
	// focus from the input after the first character.
	$effect(() => {
		// Touch dependencies (q excluded on purpose for debounce)
		filterStatus;
		filterNode;
		sortCol;
		sortDir;
		if (!browser || !hasMounted) return;
		// Defer to next tick to batch with input debounce.
		queueMicrotask(() => {
			if (currentPage !== 1) currentPage = 1;
			pushUrlState();
			if (shouldSearch) void load();
			else resetResults();
		});
	});

	// Initial mount: hydrate from URL, optionally seed node dropdown, then search if needed.
	$effect(() => {
		if (!browser || hasMounted) return;
		hasMounted = true;
		// Seed available nodes for the filter dropdown even before first search (cheap, respects visibility).
		void seedNodesIfNeeded();
		if (shouldSearch) void load();
	});

	// Cleanup timers/aborts on unmount.
	$effect(() => {
		return () => {
			clearSlowTimer();
			if (loadAbort) loadAbort.abort();
		};
	});

	function pushUrlState(): void {
		const params = new URLSearchParams();
		if (q.trim()) params.set('q', q.trim());
		if (filterStatus) params.set('status', filterStatus);
		if (filterNode) params.set('node', filterNode);
		if (sortCol !== 'vmid') params.set('sortBy', sortCol);
		if (sortDir !== 'asc') params.set('sortOrder', sortDir);
		if (currentPage > 1) params.set('page', String(currentPage));
		const url = `/search${params.size ? `?${params.toString()}` : ''}`;
		if (browser && page.url.pathname + page.url.search !== url) {
			replaceState(url, {});
		}
	}

	function resetResults(): void {
		vms = [];
		total = 0;
		totalPages = 1;
		hasNext = false;
		hasPrev = false;
		error = null;
		searched = false;
	}

	async function seedNodesIfNeeded(): Promise<void> {
		// Only seed if we have no nodes yet. This populates the node filter for first-time visitors.
		if (knownNodes.length > 0) return;
		try {
			const res = await getVMsPaginated({ page: 1, limit: 1 });
			if (res.pagination?.nodes && res.pagination.nodes.length > 0) {
				nodesFacet = res.pagination.nodes;
			}
		} catch {
			// Non-fatal; nodes will populate on first real search.
		}
	}

	async function load(): Promise<void> {
		// Cancel any in-flight request.
		if (loadAbort) loadAbort.abort();
		const abort = new AbortController();
		loadAbort = abort;

		loading = true;
		searched = true;
		error = null;
		slowLoadingVisible = false;
		startSlowTimer();

		try {
			const params: VMPaginationParams = {
				page: currentPage,
				limit: PAGE_SIZE,
				search: extractedQuery || undefined,
				type: detectedType,
				status: filterStatus || undefined,
				node: filterNode || undefined,
				sortBy: sortCol,
				sortOrder: sortDir
			};
			const res = await getVMsPaginated(params);
			if (abort.signal.aborted) return;

			vms = res.vms ?? [];
			total = res.pagination?.total ?? 0;
			totalPages = res.pagination?.totalPages ?? 1;
			hasNext = res.pagination?.hasNext ?? false;
			hasPrev = res.pagination?.hasPrev ?? false;

			// Update nodes facet for the filter dropdown from server (preferred over client derivation).
			if (res.pagination?.nodes && res.pagination.nodes.length > 0) {
				// Union with any previously known to keep options stable across narrow searches.
				const union = new Set<string>([...nodesFacet, ...res.pagination.nodes]);
				nodesFacet = Array.from(union).sort();
			}
		} catch (caught: unknown) {
			if (abort.signal.aborted) return;
			const err = caught instanceof Error ? caught : new Error(String(caught));
			error = normalizeSearchError(err);
		} finally {
			if (!abort.signal.aborted) {
				loading = false;
				slowLoadingVisible = false;
				clearSlowTimer();
				if (loadAbort === abort) loadAbort = null;
			}
		}
	}

	function normalizeSearchError(err: Error): Error {
		if (err.message === 'Search query required when searching by name or tag') {
			return new Error($t('search.queryRequired'));
		}
		return err;
	}

	function startSlowTimer(): void {
		clearSlowTimer();
		slowTimer = setTimeout(() => {
			slowLoadingVisible = true;
		}, SLOW_LOADING_DELAY_MS);
	}

	function clearSlowTimer(): void {
		if (!slowTimer) return;
		clearTimeout(slowTimer);
		slowTimer = null;
	}

	function toggleSort(col: SortColumn): void {
		if (sortCol === col) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortCol = col;
			sortDir = col === 'vmid' ? 'asc' : 'asc';
		}
		pushUrlState();
		void load();
	}

	function goToPage(p: number): void {
		if (p < 1 || p > totalPages || p === currentPage) return;
		currentPage = p;
		pushUrlState();
		void load();
	}

	function onStatusChange(): void {
		currentPage = 1;
		pushUrlState();
		if (shouldSearch) void load();
		else resetResults();
	}

	function onNodeChange(): void {
		currentPage = 1;
		pushUrlState();
		if (shouldSearch) void load();
		else resetResults();
	}

	function clearAll(): void {
		q = '';
		filterStatus = '';
		filterNode = '';
		sortCol = 'vmid';
		sortDir = 'asc';
		currentPage = 1;
		pushUrlState();
		resetResults();
	}

	// Explicit search button (immediate).
	function forceSearch(): void {
		currentPage = 1;
		pushUrlState();
		if (shouldSearch) void load();
		else resetResults();
	}

	// Allow pressing Enter in the input to force immediate search (in addition to debounce).
	function onSearchKeydown(e: KeyboardEvent): void {
		if (e.key === 'Enter') {
			e.preventDefault();
			forceSearch();
		}
		if (e.key === 'Escape' && q) {
			q = '';
			currentPage = 1;
			pushUrlState();
			if (shouldSearch) void load();
			else resetResults();
		}
	}
</script>

<svelte:head>
	<title>PVMSS — {$t('nav.searchVm')}</title>
</svelte:head>

<div class="mx-auto px-4 py-6 pv-content-width">
	<h1 class="mb-5 text-2xl font-bold">{$t('nav.searchVm')}</h1>

	<!-- Filters -->
	<div class="mb-4 grid gap-3 lg:grid-cols-[1fr_auto_auto_auto]">
		<div class="flex flex-col gap-1">
			<div class="relative">
				<MagnifyingGlass class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
				<input
					class="w-full rounded-md border border-border bg-background py-2 pl-9 pr-9 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
					placeholder={$t('search.placeholderUnified')}
					bind:value={q}
					use:debounce={{ handler: () => { currentPage = 1; pushUrlState(); if (shouldSearch) void load(); else resetResults(); }, delay: DEBOUNCE_DELAY_MS }}
					onkeydown={onSearchKeydown}
					autocomplete="off"
					spellcheck="false"
					use:autofocus
				/>
				{#if q}
					<button
						type="button"
						class="absolute right-2 top-1/2 h-6 w-6 -translate-y-1/2 rounded-md p-0.5 text-muted-foreground hover:bg-accent hover:text-foreground"
						onclick={() => { q = ''; currentPage = 1; pushUrlState(); if (shouldSearch) void load(); else resetResults(); }}
						aria-label="Clear search"
					>
						<X class="h-4 w-4" />
					</button>
				{/if}
			</div>
			<p class="text-xs text-muted-foreground">{$t('search.hintUnified')}</p>
		</div>

		<select
			class="rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
			bind:value={filterStatus}
			aria-label={$t('common.status')}
			onchange={onStatusChange}
		>
			<option value="">{$t('common.status')}</option>
			{#each STATUS_FILTERS as status (status)}
				<option value={status}>{$t(`common.statusMap.${status}`, { default: status })}</option>
			{/each}
		</select>

		<select
			class="rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
			bind:value={filterNode}
			aria-label={$t('common.node')}
			onchange={onNodeChange}
		>
			<option value="">{$t('common.node')}</option>
			{#each knownNodes as node (node)}
				<option value={node}>{node}</option>
			{/each}
		</select>

		<div class="flex gap-2">
			<button
				class="inline-flex items-center justify-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:opacity-90 disabled:opacity-50"
				onclick={forceSearch}
				disabled={loading}
			>
				{#if loading}
					<SpinnerGap class="h-4 w-4 animate-spin" />
					{$t('common.loading')}
				{:else}
					<MagnifyingGlass class="h-4 w-4" />
					{$t('search.search')}
				{/if}
			</button>
			<button
				class="inline-flex items-center justify-center gap-2 rounded-md border border-border bg-background px-3 py-2 text-sm hover:bg-accent"
				onclick={clearAll}
				disabled={loading}
			>
				{$t('common.clearFilters')}
			</button>
		</div>
	</div>

	<!-- Slow loading notice -->
	{#if slowLoadingVisible}
		<div class="mb-3 flex items-center gap-2 rounded-md border border-warning-soft-border bg-warning-soft px-3 py-2 text-sm text-warning-soft-foreground">
			<SpinnerGap class="h-4 w-4 animate-spin" />
			{$t('search.slowLoading')}
		</div>
	{/if}

	<!-- States -->
	{#if error}
		<ErrorBanner {error} onRetry={() => void load()} />
	{:else if loading && !searched}
		<LoadingSkeleton variant="card" rows={6} />
	{:else if !searched}
		<div class="py-16 text-center text-muted-foreground">
			<MagnifyingGlass class="mx-auto mb-3 h-10 w-10 opacity-30" />
			<p class="text-sm">{$t('search.hint')}</p>
		</div>
	{:else if vms.length === 0 && !loading}
		<div class="pv-table-wrap py-16 text-center text-muted-foreground">
			<Desktop class="mx-auto mb-3 h-10 w-10 opacity-30" />
			<p class="text-sm">{$t('search.noResults')}</p>
		</div>
	{:else}
		<!-- Results header -->
		<div class="mb-2 flex items-center justify-between gap-3 text-sm text-muted-foreground">
			<span>
				{$t('search.resultsCount', { values: { count: total } })}
				{#if total > PAGE_SIZE}· {$t('common.pagination.showing', { values: { start: (currentPage - 1) * PAGE_SIZE + 1, end: Math.min(currentPage * PAGE_SIZE, total), total } })}{/if}
			</span>
			{#if loading}
				<span class="inline-flex items-center gap-2">
					<SpinnerGap class="h-4 w-4 animate-spin" />
					{$t('common.loading')}
				</span>
			{/if}
		</div>

		<!-- Results table -->
		<div class="pv-table-wrap">
			<table class="pv-table">
				<thead>
					<tr>
						<th>
							<button class="sort-btn" onclick={() => toggleSort('vmid')}>
								{$t('vms.vmid')}
								{#if sortCol === 'vmid'}
									{#if sortDir === 'asc'}<ArrowUp class="sort-icon" />{:else}<ArrowDown class="sort-icon" />{/if}
								{:else}<span class="sort-icon sort-icon--inactive">↕</span>{/if}
							</button>
						</th>
						<th>
							<button class="sort-btn" onclick={() => toggleSort('name')}>
								{$t('common.name')}
								{#if sortCol === 'name'}
									{#if sortDir === 'asc'}<ArrowUp class="sort-icon" />{:else}<ArrowDown class="sort-icon" />{/if}
								{:else}<span class="sort-icon sort-icon--inactive">↕</span>{/if}
							</button>
						</th>
						<th>
							<button class="sort-btn" onclick={() => toggleSort('node')}>
								{$t('common.node')}
								{#if sortCol === 'node'}
									{#if sortDir === 'asc'}<ArrowUp class="sort-icon" />{:else}<ArrowDown class="sort-icon" />{/if}
								{:else}<span class="sort-icon sort-icon--inactive">↕</span>{/if}
							</button>
						</th>
						<th>
							<button class="sort-btn" onclick={() => toggleSort('status')}>
								{$t('common.status')}
								{#if sortCol === 'status'}
									{#if sortDir === 'asc'}<ArrowUp class="sort-icon" />{:else}<ArrowDown class="sort-icon" />{/if}
								{:else}<span class="sort-icon sort-icon--inactive">↕</span>{/if}
							</button>
						</th>
						<th>
							<button class="sort-btn" onclick={() => toggleSort('cpu')}>
								{$t('vms.cpu')}
								{#if sortCol === 'cpu'}
									{#if sortDir === 'asc'}<ArrowUp class="sort-icon" />{:else}<ArrowDown class="sort-icon" />{/if}
								{:else}<span class="sort-icon sort-icon--inactive">↕</span>{/if}
							</button>
						</th>
						<th>
							<button class="sort-btn" onclick={() => toggleSort('memory')}>
								{$t('vms.ram')}
								{#if sortCol === 'memory'}
									{#if sortDir === 'asc'}<ArrowUp class="sort-icon" />{:else}<ArrowDown class="sort-icon" />{/if}
								{:else}<span class="sort-icon sort-icon--inactive">↕</span>{/if}
							</button>
						</th>
						<th>{$t('vms.disk')}</th>
						<th>{$t('vms.uptime')}</th>
						<th>{$t('vms.tags')}</th>
					</tr>
				</thead>
				<tbody>
					{#each vms as vm (vm.vmid)}
						<tr animate:flip class="pv-row pv-row--clickable" onclick={() => goto(`/vm/${vm.vmid}`)}>
							<td class="pv-td-mono text-sm">{vm.vmid}</td>
							<td>
								<div class="pv-resource-cell">
									<div class="pv-resource-icon pv-resource-icon--vm text-[0.6rem]">VM</div>
									<span class="pv-resource-name">{vm.name || '—'}</span>
								</div>
							</td>
							<td class="pv-td-mono text-sm">{vm.node}</td>
							<td>
								<span class="pv-badge {vmList.statusClass(vm.status)}">
									{$t(`common.statusMap.${vm.status}`, { default: vm.status })}
								</span>
							</td>
							<td class="text-sm">
								{#if vm.cpu != null}
									<span class="inline-flex items-center gap-1">
										<Cpu class="h-3.5 w-3.5 opacity-60" />
										{(vm.cpu * 100).toFixed(1)}%
									</span>
								{:else}
									<span class="text-muted-foreground">—</span>
								{/if}
							</td>
							<td class="text-sm">
								{#if vm.maxMemMb}
									<span class="inline-flex items-center gap-1">
										<Memory class="h-3.5 w-3.5 opacity-60" />
										{vmList.formatMem(vm.memMb)} / {vmList.formatMem(vm.maxMemMb)}
									</span>
								{:else}
									<span class="text-muted-foreground">—</span>
								{/if}
							</td>
							<td class="text-sm">
								{#if vm.maxDiskMb}
									<span class="inline-flex items-center gap-1">
										<HardDrive class="h-3.5 w-3.5 opacity-60" />
										{vmList.formatDisk(vm.diskMb)} / {vmList.formatDisk(vm.maxDiskMb)}
									</span>
								{:else}
									<span class="text-muted-foreground">—</span>
								{/if}
							</td>
							<td class="text-sm">
								<span class="inline-flex items-center gap-1">
									<Clock class="h-3.5 w-3.5 opacity-60" />
									{vmList.uptimeLabel(vm.uptime || 0)}
								</span>
							</td>
							<td>
								{#if vm.tags}
									<div class="flex flex-wrap gap-1">
										{#each vmList.splitTags(vm.tags) as tag (tag)}
											<span class="pv-badge text-xs">{tag}</span>
										{/each}
									</div>
								{:else}
									<span class="text-muted-foreground">—</span>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<!-- Server-driven pagination -->
		{#if totalPages > 1}
			<div class="mt-4 flex flex-wrap items-center justify-center gap-1">
				<button class="pagination-btn" onclick={() => goToPage(currentPage - 1)} disabled={!hasPrev}>&lsaquo;</button>
				{#each Array.from({ length: totalPages }, (_, i) => i + 1) as p (p)}
					<button class="pagination-btn {p === currentPage ? 'pagination-btn--active' : ''}" onclick={() => goToPage(p)}>{p}</button>
				{/each}
				<button class="pagination-btn" onclick={() => goToPage(currentPage + 1)} disabled={!hasNext}>&rsaquo;</button>
			</div>
			<p class="mt-1 text-center text-xs text-muted-foreground">
				{$t('common.pageOf', { values: { page: currentPage, total: totalPages } })} · {total} total
			</p>
		{/if}
	{/if}
</div>

<style>
	.sort-btn {
		display: inline-flex;
		align-items: center;
		gap: 0.25rem;
		font-weight: 600;
		font-size: inherit;
		background: none;
		border: none;
		padding: 0;
		cursor: pointer;
		color: inherit;
		white-space: nowrap;
	}
	.sort-btn:hover { opacity: 0.7; }
	.sort-icon { width: 0.75rem; height: 0.75rem; }
	.sort-icon--inactive { opacity: 0.35; font-size: 0.7rem; }
	.pagination-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 2rem;
		height: 2rem;
		padding: 0 0.4rem;
		border-radius: 0.375rem;
		border: 1px solid var(--border);
		background: var(--background);
		color: var(--foreground);
		font-size: 0.8rem;
		cursor: pointer;
	}
	.pagination-btn:disabled { opacity: 0.4; cursor: default; }
	.pagination-btn:not(:disabled):hover { background: var(--accent); }
	.pagination-btn--active {
		background: var(--primary);
		color: var(--primary-foreground);
		border-color: var(--primary);
	}
</style>
