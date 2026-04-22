<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { t } from 'svelte-i18n';
	import { searchVMs, type VMSummary, type SearchType } from '$lib/api/vms';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import { MagnifyingGlass, Desktop, ArrowUp, ArrowDown } from 'phosphor-svelte';

	const PAGE_SIZE = 10;

	// Auto-detect search type based on input pattern
	function detectSearchType(query: string): SearchType {
		if (!query) return 'name';
		const trimmed = query.trim();
		
		// If starts with "tag:" or "#", search by tag
		if (trimmed.startsWith('tag:') || trimmed.startsWith('#')) {
			return 'tag';
		}
		
		// If all digits, search by VMID
		if (/^\d+$/.test(trimmed)) {
			return 'vmid';
		}
		
		// Otherwise search by name
		return 'name';
	}

	function extractSearchQuery(query: string): string {
		const trimmed = query.trim();
		// Remove "tag:" prefix if present
		if (trimmed.startsWith('tag:')) {
			return trimmed.substring(4).trim();
		}
		// Remove "#" prefix if present
		if (trimmed.startsWith('#')) {
			return trimmed.substring(1).trim();
		}
		return trimmed;
	}

	// --- state ---
	let q = $state($page.url.searchParams.get('q') ?? '');
	let validationError = $state<string | null>(null);

	let loading = $state(false);
	let searched = $state(false);
	let error = $state<Error | null>(null);
	let results = $state<VMSummary[]>([]);
	// Sorting
	let sortCol = $state<'name' | 'vmid'>('vmid');
	let sortDir = $state<'asc' | 'desc'>('asc');

	// Pagination
	let currentPage = $state(1);

	// Slow-loading hint
	let slowLoadingVisible = $state(false);
	let slowTimer: ReturnType<typeof setTimeout> | null = null;

	// --- derived ---
	let sortedResults = $derived.by(() => {
		const copy = [...results];
		copy.sort((a, b) => {
			const cmp = sortCol === 'name'
				? (a.name || '').localeCompare(b.name || '')
				: a.vmid - b.vmid;
			return sortDir === 'asc' ? cmp : -cmp;
		});
		return copy;
	});

	let totalPages = $derived(Math.max(1, Math.ceil(sortedResults.length / PAGE_SIZE)));

	let pagedResults = $derived(
		sortedResults.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE)
	);

	$effect(() => {
		sortedResults;
		currentPage = 1;
	});

	function onInputChange(e: Event) {
		const value = (e.target as HTMLInputElement).value;
		q = value;
		// No validation - backend handles it
		validationError = null;
	}

	async function doSearch() {
		loading = true;
		searched = true;
		error = null;
		slowLoadingVisible = false;

		if (slowTimer) { clearTimeout(slowTimer); slowTimer = null; }
		slowTimer = setTimeout(() => { slowLoadingVisible = true; }, 2500);

		const detectedType = detectSearchType(q);
		const extractedQuery = extractSearchQuery(q);
		
		const params = new URLSearchParams();
		if (extractedQuery) params.set('q', extractedQuery);
		params.set('type', detectedType);
		goto(`/search${params.toString() ? '?' + params.toString() : ''}`, { replaceState: true, noScroll: true });

		try {
			results = await searchVMs({ q: extractedQuery || undefined, type: detectedType });
		} catch (e) {
			const err = e as Error;
			if (err.message === 'Search query required when searching by name or tag') {
				error = new Error($t('search.queryRequired'));
			} else {
				error = err;
			}
		} finally {
			loading = false;
			slowLoadingVisible = false;
			if (slowTimer) { clearTimeout(slowTimer); slowTimer = null; }
		}
	}

	function statusClass(status: string) {
		if (status === 'running') return 'pv-badge--online';
		if (status === 'stopped') return 'pv-badge--offline';
		return 'pv-badge--warn';
	}

	function handleKey(e: KeyboardEvent) {
		if (e.key === 'Enter') doSearch();
	}

	function toggleSort(col: 'name' | 'vmid') {
		if (sortCol === col) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortCol = col;
			sortDir = 'asc';
		}
	}

	onMount(() => {
		if (q) doSearch();
	});

	onDestroy(() => {
		if (slowTimer) clearTimeout(slowTimer);
	});
</script>

<svelte:head>
	<title>PVMSS — {$t('nav.searchVm')}</title>
</svelte:head>

<div class="mx-auto px-4 py-6 pv-content-width">
	<h1 class="mb-5 text-2xl font-bold">{$t('nav.searchVm')}</h1>

	<!-- Search bar -->
	<div class="mb-4 flex flex-col gap-2 sm:flex-row sm:items-start">
		<div class="flex flex-1 flex-col gap-1">
			<div class="relative">
				<MagnifyingGlass class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
				<input
					class="w-full rounded-md border border-border bg-background py-2 pl-9 pr-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
					placeholder={$t('search.placeholderUnified')}
					value={q}
					oninput={onInputChange}
					onkeydown={handleKey}
					autocomplete="off"
					spellcheck="false"
				/>
			</div>
			<p class="text-xs text-muted-foreground">{$t('search.hintUnified')}</p>
		</div>

		<button
			class="inline-flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:opacity-90 disabled:opacity-50"
			onclick={doSearch}
			disabled={loading}
		>
			<MagnifyingGlass class="h-4 w-4" />
			{loading ? $t('common.loading') : $t('search.search')}
		</button>
	</div>

	<!-- Slow-loading hint -->
	{#if slowLoadingVisible}
		<div class="mb-3 flex items-center gap-2 rounded-md border border-yellow-300 bg-yellow-50 px-3 py-2 text-sm text-yellow-800 dark:border-yellow-700 dark:bg-yellow-900/20 dark:text-yellow-300">
			<span>⏳</span>
			{$t('search.slowLoading')}
		</div>
	{/if}

	<!-- Results -->
	{#if error}
		<ErrorBanner {error} onRetry={doSearch} />
	{:else if loading}
		<LoadingSkeleton variant="card" rows={4} />
	{:else if !searched}
		<div class="py-16 text-center text-muted-foreground">
			<MagnifyingGlass class="mx-auto mb-3 h-10 w-10 opacity-30" />
			<p class="text-sm">{$t('search.hint')}</p>
		</div>
	{:else if results.length === 0}
		<div class="pv-table-wrap py-16 text-center text-muted-foreground">
			<Desktop class="mx-auto mb-3 h-10 w-10 opacity-30" />
			<p class="text-sm">{$t('search.noResults')}</p>
		</div>
	{:else}
		<div class="mb-2 text-sm text-muted-foreground">
			{$t('search.resultsCount', { values: { count: results.length } })}
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
						<tr class="pv-row pv-row--clickable" onclick={() => goto(`/vm/${vm.vmid}`)}>
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

		<!-- Pagination -->
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
	:global(.pv-row--clickable) { cursor: pointer; }
	:global(.pv-row--clickable:hover td) { background: var(--accent); }

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
