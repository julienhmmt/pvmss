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
	import * as Select from '$lib/components/ui/select';
	import { getISOs, toggleISO } from '$lib/api/admin/iso';
	import { formatBytes } from '$lib/utils/format';
	import { DiscIcon, MagnifyingGlass } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { ISO } from '$lib/types/admin';

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let isos: ISO[] = $state([]);
	let selectedNodes = $state<string[]>([]);
	let searchQuery = $state<string>('');
	let toggling = $state<string | null>(null);

	const nodes = $derived([...new Set(isos.map((i) => i.node))].sort());
	const filteredISOs = $derived(
		isos.filter((i) => {
			const matchesNode = selectedNodes.length > 0 ? selectedNodes.includes(i.node) : true;
			const matchesSearch = searchQuery
				? i.name.toLowerCase().includes(searchQuery.toLowerCase())
				: true;
			return matchesNode && matchesSearch;
		})
	);
	const enabledCount = $derived(isos.filter((i) => i.enabled).length);
	const nodeFilterLabel = $derived(
		selectedNodes.length === 0
			? $t('admin.iso.allNodes')
			: selectedNodes.length === 1
				? selectedNodes[0]
				: $t('admin.iso.selectedNodes', { values: { count: selectedNodes.length } })
	);

	let page = $state(1);
	let perPage = $state(25);
	const pagedISOs = $derived(paginate(filteredISOs, page, perPage));

	$effect(() => {
		selectedNodes;
		searchQuery;
		page = 1;
	});

	async function load() {
		if (isos.length > 0) {
			refreshing = true;
		} else {
			loading = true;
		}
		error = null;
		try {
			isos = await getISOs();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	async function handleToggle(iso: ISO) {
		toggling = iso.volid;
		try {
			await toggleISO(iso.volid);
			if (iso.enabled) {
				toast.success($t('admin.iso.toast.disabled', { values: { name: iso.name } }));
			} else {
				toast.success($t('admin.iso.toast.enabled', { values: { name: iso.name } }));
			}
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		} finally {
			toggling = null;
		}
	}

	onMount(load);
</script>

<svelte:head>
	<title>PVMSS — {$t('admin.iso.title')}</title>
</svelte:head>

<PvHeader
	eyebrow={$t('nav.administration')}
	title={$t('admin.iso.title')}
	subtitle={!loading
		? `${$t('admin.iso.enabledCount', { values: { count: enabledCount } })} / ${isos.length}`
		: undefined}
>
	{#snippet stats()}
		{#if !loading && isos.length > 0}
			<PvHeaderStat label={$t('common.total')} value={isos.length} />
			<PvHeaderStat label={$t('common.enabled')} value={enabledCount} />
		{/if}
	{/snippet}
</PvHeader>

<div class="pv-content-width">

<LoadingToast visible={refreshing} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="table" rows={5} />
{:else if isos.length === 0}
	<EmptyState
		title={$t('admin.iso.noIso')}
		icon={DiscIcon}
		description={$t('admin.iso.noIsoDesc')}
	/>
{:else}
	<div class="mb-4 flex flex-wrap items-center gap-3">
		<div class="relative flex-1 min-w-[200px] max-w-[320px]">
			<MagnifyingGlass class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
			<input
				type="text"
				class="w-full rounded-md border border-border bg-background py-2 pl-9 pr-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
				placeholder={$t('admin.iso.searchPlaceholder')}
				bind:value={searchQuery}
			/>
		</div>
		{#if nodes.length > 1}
			<Select.Root
				type="multiple"
				value={selectedNodes}
				onValueChange={(v) => {
					selectedNodes = v ?? [];
				}}
			>
				<Select.Trigger class="w-[220px]">
					{nodeFilterLabel}
				</Select.Trigger>
				<Select.Content>
					{#each nodes as node}
						<Select.Item value={node}>{node}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
			{#if selectedNodes.length > 0}
				<button
					type="button"
					class="text-xs text-muted-foreground hover:text-foreground underline"
					onclick={() => (selectedNodes = [])}
				>
					{$t('common.clear')}
				</button>
			{/if}
		{/if}
	</div>

	{#if filteredISOs.length === 0}
		<div class="pv-table-wrap py-12 text-center text-muted-foreground">
			<MagnifyingGlass class="mx-auto mb-3 h-10 w-10 opacity-30" />
			<p class="text-sm">{$t('admin.iso.noSearchResults')}</p>
		</div>
	{:else}
		<div class="pv-table-wrap">
			<table class="pv-table">
				<thead>
					<tr>
						<th>{$t('common.name')}</th>
						<th>{$t('common.node')}</th>
						<th>{$t('common.storage')}</th>
						<th>{$t('common.size')}</th>
						<th class="pv-td-actions">{$t('common.enabled')}</th>
					</tr>
				</thead>
				<tbody>
					{#each pagedISOs as iso (iso.volid)}
						<tr class="pv-row" class:opacity-50={toggling === iso.volid}>
							<td>
								<div class="pv-resource-cell">
									<div class="pv-resource-icon" style="width:28px;height:28px">
										<DiscIcon class="h-3.5 w-3.5" />
									</div>
									<span class="pv-td-mono">{iso.name}</span>
								</div>
							</td>
							<td class="pv-td-muted">{iso.node}</td>
							<td class="pv-td-muted">{iso.storage}</td>
							<td class="pv-td-muted">{formatBytes(iso.size)}</td>
							<td class="pv-td-actions">
								<Switch
									checked={iso.enabled}
									disabled={toggling === iso.volid}
									onCheckedChange={() => handleToggle(iso)}
								/>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<Paginator total={filteredISOs.length} bind:page bind:perPage />
	{/if}
{/if}

</div>
