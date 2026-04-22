<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import LoadingToast from '$lib/components/data/LoadingToast.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import { Switch } from '$lib/components/ui/switch';
	import * as Select from '$lib/components/ui/select';
	import { getVMBRs, toggleVMBR } from '$lib/api/admin/vmbr';
	import { WifiHighIcon, CaretUp, CaretDown, ArrowsDownUp } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { VMBR } from '$lib/types/admin';

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	type SortKey = 'iface' | 'node' | 'type' | 'ports';
	type SortDir = 'asc' | 'desc';

	let vmbrs = $state<VMBR[]>([]);
	let selectedNode = $state<string>('');
	let sortKey = $state<SortKey>('iface');
	let sortDir = $state<SortDir>('asc');
	let toggling = $state<string | null>(null);

	const nodes = $derived([...new Set(vmbrs.map((v) => v.node))].sort());
	const filteredVmbrs = $derived(selectedNode ? vmbrs.filter((v) => v.node === selectedNode) : vmbrs);
	const sortedVmbrs = $derived(sortVmbrs(filteredVmbrs, sortKey, sortDir));
	const enabledCount = $derived(vmbrs.filter((v) => v.enabled).length);

	async function load() {
		if (vmbrs.length > 0) {
			refreshing = true;
		} else {
			loading = true;
		}
		error = null;
		try {
			vmbrs = await getVMBRs();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	async function handleToggle(iface: string, node: string, currentlyEnabled: boolean) {
		const key = node + ':' + iface;
		toggling = key;
		try {
			await toggleVMBR(iface, node);
			if (currentlyEnabled) {
				toast.success($t('admin.vmbr.toast.disabled', { values: { iface, node } }));
			} else {
				toast.success($t('admin.vmbr.toast.enabled', { values: { iface, node } }));
			}
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		} finally {
			toggling = null;
		}
	}

	function sortVmbrs(list: VMBR[], key: SortKey, dir: SortDir): VMBR[] {
		return [...list].sort((a, b) => {
			const prop = key === 'ports' ? 'bridgePorts' : key;
			const aVal = (a[prop as keyof VMBR] ?? '').toString();
			const bVal = (b[prop as keyof VMBR] ?? '').toString();
			const cmp = aVal.localeCompare(bVal);
			return dir === 'asc' ? cmp : -cmp;
		});
	}

	function toggleSort(key: SortKey): void {
		if (sortKey === key) sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		else { sortKey = key; sortDir = 'asc'; }
	}

	onMount(load);
</script>

<svelte:head>
	<title>PVMSS — {$t('admin.vmbr.title')}</title>
</svelte:head>

<!-- Gradient page header -->
<div class="pv-header -mx-6 -mt-6 mb-6">
	<div class="pv-header-flex">
		<div>
			<p class="pv-eyebrow">{$t('nav.administration')}</p>
			<h1 class="pv-title">{$t('admin.vmbr.title')}</h1>
			{#if !loading}
				<p class="pv-subtitle">
					{$t('admin.vmbr.enabledCount', { values: { count: enabledCount } })} / {vmbrs.length}
				</p>
			{/if}
		</div>

		{#if !loading && vmbrs.length > 0}
			<div class="flex items-center gap-3">
				<div class="pv-header-stats">
					<div class="pv-header-stat">
						<div class="pv-header-stat-label">{$t('common.total')}</div>
						<div class="pv-header-stat-value">{vmbrs.length}</div>
					</div>
					<div class="pv-header-stat">
						<div class="pv-header-stat-label">{$t('admin.vmbr.enabled')}</div>
						<div class="pv-header-stat-value">{enabledCount}</div>
					</div>
				</div>
			</div>
		{/if}
	</div>
</div>

<div class="pv-content-width">

<LoadingToast visible={refreshing} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="table" rows={5} />
{:else if vmbrs.length === 0}
	<EmptyState
		title={$t('admin.vmbr.noVmbr')}
		icon={WifiHighIcon}
		description={$t('admin.vmbr.noVmbrDesc')}
	/>
{:else}
	{#if nodes.length > 1}
		<div class="mb-4">
			<Select.Root
				type="single"
				value={selectedNode}
				onValueChange={(v) => {
					selectedNode = v ?? '';
				}}
			>
				<Select.Trigger class="w-[200px]">
					{selectedNode || $t('admin.vmbr.allNodes')}
				</Select.Trigger>
				<Select.Content>
					<Select.Item value="">{$t('admin.vmbr.allNodes')}</Select.Item>
					{#each nodes as node}
						<Select.Item value={node}>{node}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		</div>
	{/if}

	<div class="pv-table-wrap">
		<table class="pv-table">
			<thead>
				<tr>
					<th>
						<button class="pv-sort-btn" onclick={() => toggleSort('iface')}>
							{$t('admin.vmbr.iface')}
							{@render sortIcon('iface')}
						</button>
					</th>
					<th>
						<button class="pv-sort-btn" onclick={() => toggleSort('node')}>
							{$t('common.node')}
							{@render sortIcon('node')}
						</button>
					</th>
					<th>
						<button class="pv-sort-btn" onclick={() => toggleSort('type')}>
							{$t('common.type')}
							{@render sortIcon('type')}
						</button>
					</th>
					<th>
						<button class="pv-sort-btn" onclick={() => toggleSort('ports')}>
							{$t('admin.vmbr.ports')}
							{@render sortIcon('ports')}
						</button>
					</th>
					<th class="pv-td-actions">{$t('admin.vmbr.enabled')}</th>
				</tr>
			</thead>
			<tbody>
				{#each sortedVmbrs as v}
					{@const key = v.node + ':' + v.iface}
					<tr class="pv-row" class:opacity-50={toggling === key}>
						<td>
							<div class="pv-resource-cell">
								<div class="pv-resource-icon" style="width:28px;height:28px">
									<WifiHighIcon class="h-3.5 w-3.5" />
								</div>
								<span class="pv-td-mono">{v.iface}</span>
							</div>
						</td>
						<td class="pv-td-muted">{v.node}</td>
						<td class="pv-td-muted">{v.type || '—'}</td>
						<td class="pv-td-muted">{v.bridgePorts || '—'}</td>
						<td class="pv-td-actions">
							<Switch
								checked={v.enabled}
								disabled={toggling === key}
								onCheckedChange={() => handleToggle(v.iface, v.node, v.enabled)}
							/>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}

</div>

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
