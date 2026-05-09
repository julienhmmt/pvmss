<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import LoadingToast from '$lib/components/data/LoadingToast.svelte';
	import PvHeader from '$lib/components/layout/PvHeader.svelte';
	import PvHeaderStat from '$lib/components/layout/PvHeaderStat.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import Paginator from '$lib/components/data/Paginator.svelte';
	import { paginate } from '$lib/utils/paginate';
	import { Switch } from '$lib/components/ui/switch';
	import { Button } from '$lib/components/ui/button';
	import { getStorages, toggleStorage } from '$lib/api/admin/storage';
	import { formatBytes, formatPercent } from '$lib/utils/format';
	import { Database, ArrowsClockwise, CaretUp, CaretDown, ArrowsDownUp } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { Storage } from '$lib/types/admin';
	import * as Select from '$lib/components/ui/select';

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let storages = $state<Storage[]>([]);
	let selectedNode = $state<string>('');
	let selectedType = $state<string>('');
	let toggling = $state<Set<string>>(new Set());

	type SortKey = 'name' | 'total' | 'used' | 'usage';
	type SortDir = 'asc' | 'desc';
	let sortKey = $state<SortKey>('usage');
	let sortDir = $state<SortDir>('desc');

	const nodes = $derived([...new Set(storages.map((s) => s.node))].sort());
	const storageTypes = $derived([...new Set(storages.map((s) => s.type))].sort());

	const filteredStorages = $derived(
		storages.filter((s) => {
			const nodeMatch = selectedNode ? s.node === selectedNode : true;
			const typeMatch = selectedType ? s.type === selectedType : true;
			return nodeMatch && typeMatch;
		})
	);

	const sortedStorages = $derived(sortStorages(filteredStorages, sortKey, sortDir));

	const enabledCount = $derived(filteredStorages.filter((s) => s.enabled).length);

	let page = $state(1);
	let perPage = $state(25);
	const pagedStorages = $derived(paginate(sortedStorages, page, perPage));

	$effect(() => {
		selectedNode;
		selectedType;
		page = 1;
	});

	function usageBarClass(used: number, total: number): string {
		const pct = Number(formatPercent(used, total));
		if (pct >= 80) return 'pv-usage-bar-fill--danger';
		if (pct >= 60) return 'pv-usage-bar-fill--warn';
		return '';
	}

	function storageKey(storage: string, node: string): string {
		return `${node}/${storage}`;
	}

	function sortStorages(list: Storage[], key: SortKey, dir: SortDir): Storage[] {
		return [...list].sort((a, b) => {
			let cmp = 0;
			if (key === 'usage') {
				const aPct = a.total > 0 ? a.used / a.total : 0;
				const bPct = b.total > 0 ? b.used / b.total : 0;
				cmp = aPct - bPct;
			} else if (key === 'name') {
				cmp = a.storage.localeCompare(b.storage);
			} else {
				cmp = (a[key] ?? 0) - (b[key] ?? 0);
			}
			return dir === 'asc' ? cmp : -cmp;
		});
	}

	function toggleSort(key: SortKey): void {
		if (sortKey === key) sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		else { sortKey = key; sortDir = 'asc'; }
	}

	async function load() {
		if (storages.length > 0) {
			refreshing = true;
		} else {
			loading = true;
		}
		error = null;
		try {
			storages = await getStorages();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	async function handleToggle(storage: string, node: string) {
		const key = storageKey(storage, node);
		toggling = new Set([...toggling, key]);
		try {
			await toggleStorage(storage, node);
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		} finally {
			const next = new Set(toggling);
			next.delete(key);
			toggling = next;
		}
	}

	onMount(load);
</script>

<svelte:head>
	<title>PVMSS — {$t('admin.storage.title')}</title>
</svelte:head>

<PvHeader title={$t('admin.storage.title')}>
	{#snippet stats()}
		{#if !loading}
			<PvHeaderStat label={$t('admin.storage.title')} value={filteredStorages.length} />
			{#if enabledCount > 0}
				<PvHeaderStat label={$t('common.enabled')} value={enabledCount} />
			{/if}
			{#if nodes.length > 1}
				<PvHeaderStat label={$t('nav.nodes')} value={nodes.length} />
			{/if}
		{/if}
	{/snippet}
	{#snippet actions()}
		{#if !loading}
			<Button class="pv-header-btn" variant="outline" size="sm" onclick={load} disabled={loading}>
				<ArrowsClockwise class="mr-1 h-4 w-4 {loading ? 'animate-spin' : ''}" />
				{$t('common.refresh')}
			</Button>
		{/if}
	{/snippet}
</PvHeader>

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

<style>
	:global(.pv-sort-btn) {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		background: none;
		border: none;
		font: inherit;
		font-weight: 500;
		color: inherit;
		cursor: pointer;
		padding: 0;
		white-space: nowrap;
	}
	:global(.pv-sort-btn:hover) {
		color: var(--foreground);
	}
</style>

<div class="pv-content-width">

<LoadingToast visible={refreshing} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="table" rows={5} />
{:else if storages.length === 0}
	<EmptyState title={$t('admin.storage.noStorage')} icon={Database} />
{:else}
	<!-- Toolbar -->
	<div class="pv-toolbar">
		{#if nodes.length > 1}
			<Select.Root type="single" value={selectedNode} onValueChange={(v) => (selectedNode = v ?? '')}>
				<Select.Trigger class="w-[180px] h-8 text-sm">
					{selectedNode || $t('admin.storage.allNodes')}
				</Select.Trigger>
				<Select.Content>
					<Select.Item value="">{$t('admin.storage.allNodes')}</Select.Item>
					{#each nodes as node}
						<Select.Item value={node}>{node}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		{/if}
		{#if storageTypes.length > 1}
			<Select.Root type="single" value={selectedType} onValueChange={(v) => (selectedType = v ?? '')}>
				<Select.Trigger class="w-[180px] h-8 text-sm">
					{selectedType || $t('admin.storage.allTypes')}
				</Select.Trigger>
				<Select.Content>
					<Select.Item value="">{$t('admin.storage.allTypes')}</Select.Item>
					{#each storageTypes as st}
						<Select.Item value={st}>{st}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		{/if}
	</div>

	<div class="pv-table-wrap">
		<table class="pv-table">
			<thead>
				<tr>
					<th>
						<button class="pv-sort-btn" onclick={() => toggleSort('name')}>
							{$t('common.name')}
							{@render sortIcon('name')}
						</button>
					</th>
					<th>{$t('common.type')}</th>
					<th>{$t('admin.storage.content')}</th>
					{#if nodes.length > 1 || selectedNode === ''}
						<th>{$t('common.node')}</th>
					{/if}
					<th class="pv-th-num">
						<button class="pv-sort-btn" onclick={() => toggleSort('total')}>
							{$t('admin.storage.total')}
							{@render sortIcon('total')}
						</button>
					</th>
					<th class="pv-th-num">
						<button class="pv-sort-btn" onclick={() => toggleSort('used')}>
							{$t('admin.storage.used')}
							{@render sortIcon('used')}
						</button>
					</th>
					<th>
						<button class="pv-sort-btn" onclick={() => toggleSort('usage')}>
							{$t('admin.storage.usage')}
							{@render sortIcon('usage')}
						</button>
					</th>
					<th class="text-center">{$t('common.enabled')}</th>
				</tr>
			</thead>
			<tbody>
				{#each pagedStorages as s (storageKey(s.storage, s.node))}
					{@const pct = Number(formatPercent(s.used, s.total))}
					{@const key = storageKey(s.storage, s.node)}
					<tr class="pv-row">
						<!-- Name -->
						<td>
							<div class="pv-resource-cell">
								<div class="pv-resource-icon pv-resource-icon--storage">
									{s.storage.slice(0, 2).toUpperCase()}
								</div>
								<div>
									<div class="pv-resource-name">{s.storage}</div>
									{#if s.node && nodes.length <= 1}
										<div class="pv-td-muted text-xs">{s.node}</div>
									{/if}
								</div>
							</div>
						</td>

						<!-- Type -->
						<td>
							<span class="pv-td-mono">{s.type}</span>
						</td>

						<!-- Content tags -->
						<td>
							<div class="flex flex-wrap gap-1">
								{#each (s.content ?? '').split(',').filter(Boolean) as ct}
									<span class="pv-action-badge pv-action-badge--vm text-[0.65rem]">{ct.trim()}</span>
								{/each}
							</div>
						</td>

						<!-- Node (only when showing all nodes) -->
						{#if nodes.length > 1 || selectedNode === ''}
							<td class="pv-td-muted">{s.node}</td>
						{/if}

						<!-- Total -->
						<td class="pv-td-num">
							{#if s.total > 0}
								{formatBytes(s.total)}
							{:else}
								<span class="pv-td-muted">—</span>
							{/if}
						</td>

						<!-- Used -->
						<td class="pv-td-num">
							{#if s.used > 0}
								{formatBytes(s.used)}
							{:else}
								<span class="pv-td-muted">—</span>
							{/if}
						</td>

						<!-- Usage bar -->
						<td>
							{#if s.total > 0}
								<div class="pv-usage-bar" style="min-width: 100px;">
									<div class="pv-usage-bar-track" style="flex: 1;">
										<div
											class="pv-usage-bar-fill {usageBarClass(s.used, s.total)}"
											style="width: {pct}%"
										></div>
									</div>
									<span class="pv-usage-label">{pct}%</span>
								</div>
							{:else}
								<span class="pv-td-muted text-xs">—</span>
							{/if}
						</td>

						<!-- Toggle -->
						<td class="text-center">
							<Switch
								checked={s.enabled}
								disabled={toggling.has(key)}
								onCheckedChange={() => handleToggle(s.storage, s.node)}
							/>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>

	<Paginator total={filteredStorages.length} bind:page bind:perPage />
{/if}

</div>
