<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import LoadingToast from '$lib/components/data/LoadingToast.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import { Switch } from '$lib/components/ui/switch';
	import * as Select from '$lib/components/ui/select';
	import { getISOs, toggleISO } from '$lib/api/admin/iso';
	import { formatBytes } from '$lib/utils/format';
	import { DiscIcon } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { ISO } from '$lib/types/admin';

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let isos: ISO[] = $state([]);
	let selectedNode = $state<string>('');
	let toggling = $state<string | null>(null);

	const nodes = $derived([...new Set(isos.map((i) => (i as any).node))].sort());
	const filteredISOs = $derived(selectedNode ? isos.filter((i) => (i as any).node === selectedNode) : isos);
	const enabledCount = $derived(isos.filter((i) => i.enabled).length);

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

<!-- Gradient page header -->
<div class="pv-header -mx-6 -mt-6 mb-6">
	<div class="pv-header-flex">
		<div>
			<p class="pv-eyebrow">{$t('nav.administration')}</p>
			<h1 class="pv-title">{$t('admin.iso.title')}</h1>
			{#if !loading}
				<p class="pv-subtitle">
					{$t('admin.iso.enabledCount', { values: { count: enabledCount } })} / {isos.length}
				</p>
			{/if}
		</div>

		{#if !loading && isos.length > 0}
			<div class="flex items-center gap-3">
				<div class="pv-header-stats">
					<div class="pv-header-stat">
						<div class="pv-header-stat-label">{$t('common.total')}</div>
						<div class="pv-header-stat-value">{isos.length}</div>
					</div>
					<div class="pv-header-stat">
						<div class="pv-header-stat-label">{$t('common.enabled')}</div>
						<div class="pv-header-stat-value">{enabledCount}</div>
					</div>
				</div>
			</div>
		{/if}
	</div>
</div>

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
					{selectedNode || $t('admin.iso.allNodes')}
				</Select.Trigger>
				<Select.Content>
					<Select.Item value="">{$t('admin.iso.allNodes')}</Select.Item>
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
					<th>{$t('common.name')}</th>
					<th>{$t('common.node')}</th>
					<th>{$t('common.storage')}</th>
					<th>{$t('common.size')}</th>
					<th class="pv-td-actions">{$t('common.enabled')}</th>
				</tr>
			</thead>
			<tbody>
				{#each filteredISOs as iso}
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
{/if}
