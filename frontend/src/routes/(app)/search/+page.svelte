<script lang="ts">
	import { browser } from '$app/environment';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { flip } from 'svelte/animate';
	import { t } from 'svelte-i18n';
	import { debounce } from '$lib/actions';
	import { searchVMs, type SearchType, type VMSummary } from '$lib/api/vms';
	import type { VMStatus } from '$lib/types/vm';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import { ArrowDown, ArrowUp, Desktop, MagnifyingGlass, SpinnerGap, X } from 'phosphor-svelte';

	const PAGE_SIZE = 10;
	const DEBOUNCE_DELAY_MS = 400;
	const SLOW_LOADING_DELAY_MS = 2500;
	const STATUS_FILTERS: readonly VMStatus[] = ['running', 'stopped', 'paused'];

	type SortColumn = 'name' | 'vmid';
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

	function getInitialStatus(): VMStatus | '' {
		const status = page.url.searchParams.get('status') ?? '';
		return isVMStatus(status) ? status : '';
	}

	let q = $state(page.url.searchParams.get('q') ?? '');
	let filterStatus = $state<VMStatus | ''>(getInitialStatus());
	let filterNode = $state(page.url.searchParams.get('node') ?? '');
	let loading = $state(false);
	let searched = $state(false);
	let error = $state<Error | null>(null);
	let results = $state<VMSummary[]>([]);
	let sortCol = $state<SortColumn>('vmid');
	let sortDir = $state<SortDirection>('asc');
	let currentPage = $state(1);
	let slowLoadingVisible = $state(false);
	let hasMounted = $state(false);
	let requestSeq = 0;
	let slowTimer: ReturnType<typeof setTimeout> | null = null;
	let isTriggeringSearch = $state(false);

	const extractedQuery = $derived(extractSearchQuery(q));
	const detectedType = $derived(detectSearchType(q));
	const knownNodes = $derived([...new Set(results.map((vm) => vm.node).filter(Boolean))].sort());
	const sortedResults = $derived.by(() => {
		const copy = [...results];
		copy.sort((left, right) => {
			const cmp = sortCol === 'name'
				? (left.name || '').localeCompare(right.name || '')
				: left.vmid - right.vmid;
			return sortDir === 'asc' ? cmp : -cmp;
		});
		return copy;
	});
	const totalPages = $derived(Math.max(1, Math.ceil(sortedResults.length / PAGE_SIZE)));
	const pagedResults = $derived(
		sortedResults.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE)
	);
	const shouldSearch = $derived(Boolean(extractedQuery || filterStatus || filterNode));

	$effect(() => {
		sortedResults;
		currentPage = 1;
	});

	$effect(() => {
		if (!browser || hasMounted) return;
		hasMounted = true;
		if (!isTriggeringSearch) void triggerSearch();
	});

	// Cleanup slow timer on unmount
	$effect(() => {
		return () => {
			clearSlowTimer();
		};
	});

	function triggerSearch(): void {
		if (isTriggeringSearch) return;
		isTriggeringSearch = true;

		const params = new URLSearchParams();
		if (q.trim()) params.set('q', q.trim());
		if (filterStatus) params.set('status', filterStatus);
		if (filterNode) params.set('node', filterNode);
		const url = `/search${params.size ? `?${params.toString()}` : ''}`;
		if (browser && page.url.pathname + page.url.search !== url) {
			goto(url, { replaceState: true, noScroll: true });
		}
		if (!shouldSearch) {
			results = [];
			error = null;
			searched = false;
			isTriggeringSearch = false;
			return;
		}
		void doSearch();
	}

	async function doSearch(): Promise<void> {
		const seq = ++requestSeq;
		loading = true;
		searched = true;
		error = null;
		slowLoadingVisible = false;
		startSlowTimer();
		try {
			const found = await searchVMs({
				q: extractedQuery || undefined,
				type: detectedType,
				status: filterStatus || undefined,
				node: filterNode || undefined
			});
			if (seq === requestSeq) results = found;
		} catch (caught: unknown) {
			const err = caught instanceof Error ? caught : new Error(String(caught));
			if (seq === requestSeq) error = normalizeSearchError(err);
		} finally {
			if (seq === requestSeq) {
				loading = false;
				slowLoadingVisible = false;
				clearSlowTimer();
				isTriggeringSearch = false;
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

	function statusClass(status: string): string {
		if (status === 'running') return 'pv-badge--online';
		if (status === 'stopped') return 'pv-badge--offline';
		return 'pv-badge--warn';
	}

	function toggleSort(col: SortColumn): void {
		if (sortCol === col) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
			return;
		}
		sortCol = col;
		sortDir = 'asc';
	}

	function clearSearch(): void {
		q = '';
		filterStatus = '';
		filterNode = '';
		void triggerSearch();
	}
</script>

<svelte:head>
	<title>PVMSS — {$t('nav.searchVm')}</title>
</svelte:head>

<div class="mx-auto px-4 py-6 pv-content-width">
	<h1 class="mb-5 text-2xl font-bold">{$t('nav.searchVm')}</h1>
	<div class="mb-4 grid gap-3 lg:grid-cols-[1fr_auto_auto_auto]">
		<div class="flex flex-col gap-1">
			<div class="relative">
				<MagnifyingGlass class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
				<input
					class="w-full rounded-md border border-border bg-background py-2 pl-9 pr-9 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
					placeholder={$t('search.placeholderUnified')}
					bind:value={q}
					use:debounce={{ handler: triggerSearch, delay: DEBOUNCE_DELAY_MS }}
					autocomplete="off"
					spellcheck="false"
				/>
				{#if q}
					<button
						type="button"
						class="absolute right-2 top-1/2 h-6 w-6 -translate-y-1/2 rounded-md p-0.5 text-muted-foreground hover:bg-accent hover:text-foreground"
						onclick={clearSearch}
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
			onchange={triggerSearch}
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
			onchange={triggerSearch}
		>
			<option value="">{$t('common.node')}</option>
			{#each knownNodes as node (node)}
				<option value={node}>{node}</option>
			{/each}
		</select>
		<button
			class="inline-flex items-center justify-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:opacity-90 disabled:opacity-50"
			onclick={doSearch}
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
	</div>
	{#if slowLoadingVisible}
		<div
			class="mb-3 flex items-center gap-2 rounded-md border border-yellow-300 bg-yellow-50 px-3 py-2 text-sm text-yellow-800 dark:border-yellow-700 dark:bg-yellow-900/20 dark:text-yellow-300"
		>
			<SpinnerGap class="h-4 w-4 animate-spin" />
			{$t('search.slowLoading')}
		</div>
	{/if}
	{#if error}
		<ErrorBanner {error} onRetry={doSearch} />
	{:else if loading && !searched}
		<LoadingSkeleton variant="card" rows={4} />
	{:else if !searched}
		<div class="py-16 text-center text-muted-foreground">
			<MagnifyingGlass class="mx-auto mb-3 h-10 w-10 opacity-30" />
			<p class="text-sm">{$t('search.hint')}</p>
		</div>
	{:else if results.length === 0 && !loading}
		<div class="pv-table-wrap py-16 text-center text-muted-foreground">
			<Desktop class="mx-auto mb-3 h-10 w-10 opacity-30" />
			<p class="text-sm">{$t('search.noResults')}</p>
		</div>
	{:else}
		<div class="mb-2 flex items-center justify-between gap-3 text-sm text-muted-foreground">
			<span>{$t('search.resultsCount', { values: { count: results.length } })}</span>
			{#if loading}
				<span class="inline-flex items-center gap-2">
					<SpinnerGap class="h-4 w-4 animate-spin" />
					{$t('common.loading')}
				</span>
			{/if}
		</div>
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
						<th>{$t('common.node')}</th>
						<th>{$t('common.status')}</th>
						<th>{$t('admin.vms.tags')}</th>
					</tr>
				</thead>
				<tbody>
					{#each pagedResults as vm (vm.vmid)}
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
								<span class="pv-badge {statusClass(vm.status)}">
									{$t(`common.statusMap.${vm.status}`, { default: vm.status })}
								</span>
							</td>
							<td>
								{#if vm.tags}
									<div class="flex flex-wrap gap-1">
										{#each vm.tags.split(';').filter((tag) => tag.trim() && tag.trim() !== 'pvmss') as tag (tag)}
											<span class="pv-badge text-xs">{tag.trim()}</span>
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
		{#if totalPages > 1}
			<div class="mt-4 flex items-center justify-center gap-1">
				<button class="pagination-btn" onclick={() => (currentPage = Math.max(1, currentPage - 1))} disabled={currentPage === 1}>&lsaquo;</button>
				{#each Array.from({ length: totalPages }, (_, i) => i + 1) as p (p)}
					<button class="pagination-btn {p === currentPage ? 'pagination-btn--active' : ''}" onclick={() => (currentPage = p)}>{p}</button>
				{/each}
				<button class="pagination-btn" onclick={() => (currentPage = Math.min(totalPages, currentPage + 1))} disabled={currentPage === totalPages}>&rsaquo;</button>
			</div>
			<p class="mt-1 text-center text-xs text-muted-foreground">
				{currentPage} / {totalPages}
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
