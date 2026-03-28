<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { t } from 'svelte-i18n';
	import { api } from '$lib/api/client';
	import { searchVMs, type VMSummary } from '$lib/api/vms';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import { MagnifyingGlass, Play, Stop, ArrowCounterClockwise, Desktop } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';

	// --- filters (synced with URL query string) ---
	let q = $state($page.url.searchParams.get('q') ?? '');
	let filterStatus = $state($page.url.searchParams.get('status') ?? '');
	let filterNode = $state($page.url.searchParams.get('node') ?? '');

	let loading = $state(false);
	let searched = $state(false);
	let error = $state<Error | null>(null);
	let results = $state<VMSummary[]>([]);
	let actionLoading = $state<Record<number, boolean>>({});

	// Unique node list derived from results
	let knownNodes = $state<string[]>([]);

	async function doSearch() {
		loading = true;
		searched = true;
		error = null;
		// Sync URL
		const params = new URLSearchParams();
		if (q) params.set('q', q);
		if (filterStatus) params.set('status', filterStatus);
		if (filterNode) params.set('node', filterNode);
		goto(`/search${params.toString() ? '?' + params.toString() : ''}`, { replaceState: true, noScroll: true });
		try {
			results = await searchVMs({ q: q || undefined, status: filterStatus || undefined, node: filterNode || undefined });
			// Collect nodes seen for the node filter dropdown
			const nodeSet = new Set(results.map((v) => v.node).filter(Boolean));
			if (knownNodes.length === 0) {
				knownNodes = [...nodeSet];
			} else {
				// Merge without losing previously known nodes
				for (const n of nodeSet) knownNodes = knownNodes.includes(n) ? knownNodes : [...knownNodes, n];
			}
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
		}
	}

	async function doAction(vm: VMSummary, action: string) {
		actionLoading = { ...actionLoading, [vm.vmid]: true };
		try {
			await api.post(`/api/v1/vms/${vm.vmid}/action`, { action, node: vm.node });
			toast.success(`${action} sent to ${vm.name || vm.vmid}`);
			setTimeout(() => doSearch(), 2000);
		} catch {
			toast.error(`Failed to ${action} VM ${vm.vmid}`);
		} finally {
			actionLoading = { ...actionLoading, [vm.vmid]: false };
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

	// Run search immediately if URL already has params
	onMount(() => {
		if (q || filterStatus || filterNode) doSearch();
	});
</script>

<div class="mx-auto max-w-5xl px-4 py-6">
	<h1 class="mb-5 text-2xl font-bold">{$t('nav.searchVm')}</h1>

	<!-- Search bar -->
	<div class="mb-4 flex flex-col gap-2 sm:flex-row">
		<div class="relative flex-1">
			<MagnifyingGlass class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
			<input
				class="w-full rounded-md border border-border bg-background py-2 pl-9 pr-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
				placeholder={$t('search.placeholder')}
				bind:value={q}
				onkeydown={handleKey}
			/>
		</div>

		<!-- Status filter -->
		<select
			class="rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
			bind:value={filterStatus}
		>
			<option value="">{$t('search.allStatuses')}</option>
			<option value="running">{$t('common.statusMap.running')}</option>
			<option value="stopped">{$t('common.statusMap.stopped')}</option>
			<option value="paused">{$t('common.statusMap.paused')}</option>
		</select>

		<!-- Node filter (populated after first search) -->
		{#if knownNodes.length > 0}
			<select
				class="rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
				bind:value={filterNode}
			>
				<option value="">{$t('search.allNodes')}</option>
				{#each knownNodes as node (node)}
					<option value={node}>{node}</option>
				{/each}
			</select>
		{/if}

		<button
			class="inline-flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:opacity-90 disabled:opacity-50"
			onclick={doSearch}
			disabled={loading}
		>
			<MagnifyingGlass class="h-4 w-4" />
			{loading ? $t('common.loading') : $t('search.search')}
		</button>
	</div>

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
						<th>{$t('vms.vmid')}</th>
						<th>{$t('common.name')}</th>
						<th>{$t('common.node')}</th>
						<th>{$t('common.status')}</th>
						<th>{$t('admin.vms.tags')}</th>
						<th></th>
					</tr>
				</thead>
				<tbody>
					{#each results as vm (vm.vmid)}
						{@const busy = actionLoading[vm.vmid] ?? false}
						<tr
							class="pv-row pv-row--clickable"
							onclick={() => goto(`/vm/${vm.vmid}`)}
						>
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
										{#each vm.tags.split(';').filter((t) => t.trim() && t.trim() !== 'pvmss') as tag (tag)}
											<span class="pv-badge text-xs">{tag.trim()}</span>
										{/each}
									</div>
								{:else}
									<span class="text-muted-foreground">—</span>
								{/if}
							</td>
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
	{/if}
</div>

<style>
	:global(.pv-row--clickable) {
		cursor: pointer;
	}
	:global(.pv-row--clickable:hover td) {
		background: var(--accent);
	}
</style>
