<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import PvHeader from '$lib/components/layout/PvHeader.svelte';
	import PvHeaderStat from '$lib/components/layout/PvHeaderStat.svelte';
	import LoadingToast from '$lib/components/data/LoadingToast.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import Paginator from '$lib/components/data/Paginator.svelte';
	import { paginate } from '$lib/utils/paginate';
	import { Switch } from '$lib/components/ui/switch';
	import * as Select from '$lib/components/ui/select';
	import { getVMBRs, toggleVMBR } from '$lib/api/admin/vmbr';
	import { WifiHighIcon, CaretUp, CaretDown, ArrowsDownUp, MagnifyingGlass, PlugsConnectedIcon } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { VMBR } from '$lib/types/admin';

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	type SortKey = 'iface' | 'node' | 'ports' | 'description';
	type SortDir = 'asc' | 'desc';

	let vmbrs = $state<VMBR[]>([]);
	let selectedNode = $state<string>('');
	let searchQuery = $state<string>('');
	let sortKey = $state<SortKey>('iface');
	let sortDir = $state<SortDir>('asc');
	let toggling = $state<string | null>(null);

	const nodes = $derived([...new Set(vmbrs.map((v) => v.node))].sort());
	const filteredVmbrs = $derived(
		vmbrs.filter((v) => {
			const matchesNode = selectedNode ? v.node === selectedNode : true;
			if (!matchesNode) return false;
			if (!searchQuery) return true;
			const q = searchQuery.toLowerCase();
			return (
				v.iface.toLowerCase().includes(q) ||
				(v.comments ?? '').toLowerCase().includes(q) ||
				(v.bridgePorts ?? '').toLowerCase().includes(q)
			);
		})
	);
	const sortedVmbrs = $derived(sortVmbrs(filteredVmbrs, sortKey, sortDir));
	const enabledCount = $derived(vmbrs.filter((v) => v.enabled).length);
	const activeCount = $derived(vmbrs.filter((v) => v.active).length);

	let page = $state(1);
	let perPage = $state(25);
	const pagedVmbrs = $derived(paginate(sortedVmbrs, page, perPage));

	$effect(() => {
		selectedNode;
		searchQuery;
		page = 1;
	});

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
			const prop =
				key === 'ports' ? 'bridgePorts' : key === 'description' ? 'comments' : key;
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

<PvHeader
	eyebrow={$t('nav.administration')}
	title={$t('admin.vmbr.title')}
	subtitle={!loading
		? `${$t('admin.vmbr.enabledCount', { values: { count: enabledCount } })} / ${vmbrs.length}`
		: undefined}
>
	{#snippet stats()}
		{#if !loading && vmbrs.length > 0}
			<PvHeaderStat label={$t('common.total')} value={vmbrs.length} />
			<PvHeaderStat label={$t('admin.vmbr.active')} value={activeCount} />
			<PvHeaderStat label={$t('admin.vmbr.enabled')} value={enabledCount} />
		{/if}
	{/snippet}
</PvHeader>

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
	<div class="mb-4 flex flex-wrap items-center gap-3">
		<div class="relative flex-1 min-w-[200px] max-w-[360px]">
			<MagnifyingGlass class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
			<input
				type="text"
				class="w-full rounded-md border border-border bg-background py-2 pl-9 pr-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
				placeholder={$t('admin.vmbr.searchPlaceholder')}
				bind:value={searchQuery}
			/>
		</div>
		{#if nodes.length > 1}
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
					{#each nodes as node, i (i)}
						<Select.Item value={node}>{node}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		{/if}
		{#if searchQuery || selectedNode}
			<button
				type="button"
				class="text-xs text-muted-foreground hover:text-foreground underline"
				onclick={() => {
					searchQuery = '';
					selectedNode = '';
				}}
			>
				{$t('common.clear')}
			</button>
		{/if}
	</div>

	{#if filteredVmbrs.length === 0}
		<div class="pv-table-wrap py-12 text-center text-muted-foreground">
			<MagnifyingGlass class="mx-auto mb-3 h-10 w-10 opacity-30" />
			<p class="text-sm">{$t('admin.vmbr.noSearchResults')}</p>
		</div>
	{:else}
		<div class="pv-table-wrap">
			<table class="pv-table pv-vmbr-table">
				<thead>
					<tr>
						<th class="pv-vmbr-col-iface">
							<button class="pv-sort-btn" onclick={() => toggleSort('iface')}>
								{$t('admin.vmbr.iface')}
								{@render sortIcon('iface')}
							</button>
						</th>
						<th>
							<button class="pv-sort-btn" onclick={() => toggleSort('description')}>
								{$t('admin.vmbr.description')}
								{@render sortIcon('description')}
							</button>
						</th>
						<th>
							<button class="pv-sort-btn" onclick={() => toggleSort('node')}>
								{$t('common.node')}
								{@render sortIcon('node')}
							</button>
						</th>
						<th>
							<button class="pv-sort-btn" onclick={() => toggleSort('ports')}>
								{$t('admin.vmbr.ports')}
								{@render sortIcon('ports')}
							</button>
						</th>
						<th>{$t('common.status')}</th>
						<th class="pv-td-actions">{$t('admin.vmbr.enabled')}</th>
					</tr>
				</thead>
				<tbody>
					{#each pagedVmbrs as v (v.node + ':' + v.iface)}
						{@const key = v.node + ':' + v.iface}
						<tr class="pv-row" class:opacity-50={toggling === key}>
							<td>
								<div class="pv-resource-cell">
									<div
										class="pv-resource-icon pv-vmbr-icon"
										class:is-enabled={v.enabled}
										style="width:32px;height:32px"
									>
										<WifiHighIcon class="h-4 w-4" weight="bold" />
									</div>
									<div class="flex flex-col gap-0.5 min-w-0">
										<span class="pv-td-mono font-semibold">{v.iface}</span>
										{#if v.type}
											<span class="pv-vmbr-type">{v.type}</span>
										{/if}
									</div>
								</div>
							</td>
							<td>
								{#if v.comments}
									<span class="pv-vmbr-desc" title={v.comments}>{v.comments}</span>
								{:else}
									<span class="pv-td-muted italic text-xs">{$t('admin.vmbr.noDescription')}</span>
								{/if}
							</td>
							<td class="pv-td-muted">{v.node}</td>
							<td>
								{#if v.bridgePorts?.trim()}
									<div class="flex flex-wrap gap-1">
										{#each v.bridgePorts.split(/\s+/).filter(Boolean) as port, i (i)}
											<span class="pv-port-chip">
												<PlugsConnectedIcon class="h-3 w-3" />
												{port}
											</span>
										{/each}
									</div>
								{:else}
									<span class="pv-td-muted italic text-xs">{$t('admin.vmbr.noPorts')}</span>
								{/if}
							</td>
							<td>
								{#if v.active}
									<span class="pv-badge pv-badge--online">{$t('admin.vmbr.active')}</span>
								{:else}
									<span class="pv-badge pv-badge--offline">{$t('admin.vmbr.inactive')}</span>
								{/if}
							</td>
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

		<Paginator total={filteredVmbrs.length} bind:page bind:perPage />
	{/if}
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

	:global(.pv-vmbr-table .pv-vmbr-col-iface) {
		min-width: 160px;
	}

	:global(.pv-vmbr-icon) {
		background: hsl(var(--muted));
		color: hsl(var(--muted-foreground));
		transition: all 0.15s ease;
	}
	:global(.pv-vmbr-icon.is-enabled) {
		background: hsl(var(--blaze-orange-50, 33 100% 96%));
		color: hsl(var(--blaze-orange-700, 22 90% 40%));
	}
	:global(.dark .pv-vmbr-icon.is-enabled) {
		background: hsl(22 70% 20% / 0.4);
		color: hsl(33 90% 65%);
	}

	:global(.pv-vmbr-type) {
		font-size: 0.68rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: hsl(var(--muted-foreground));
	}

	:global(.pv-vmbr-desc) {
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
		text-overflow: ellipsis;
		font-size: 0.85rem;
		line-height: 1.35;
		color: hsl(var(--foreground));
		max-width: 320px;
	}

	:global(.pv-port-chip) {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 2px 8px;
		font-size: 0.72rem;
		font-family:
			ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
		font-weight: 500;
		background: hsl(var(--muted));
		color: hsl(var(--foreground));
		border: 1px solid hsl(var(--border));
		border-radius: 6px;
		white-space: nowrap;
	}
</style>
