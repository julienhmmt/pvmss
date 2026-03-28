<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import LoadingToast from '$lib/components/data/LoadingToast.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import { Switch } from '$lib/components/ui/switch';
	import { Button } from '$lib/components/ui/button';
	import { getStorages, toggleStorage } from '$lib/api/admin/storage';
	import { formatBytes, formatPercent } from '$lib/utils/format';
	import { Database, ArrowsClockwise } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { Storage } from '$lib/types/admin';
	import * as Select from '$lib/components/ui/select';

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let storages = $state<Storage[]>([]);
	let selectedNode = $state<string>('');
	let toggling = $state<Set<string>>(new Set());

	const nodes = $derived([...new Set(storages.map((s) => s.node))].sort());

	const filteredStorages = $derived(
		selectedNode ? storages.filter((s) => s.node === selectedNode) : storages
	);

	const enabledCount = $derived(filteredStorages.filter((s) => s.enabled).length);

	function usageBarClass(used: number, total: number): string {
		const pct = Number(formatPercent(used, total));
		if (pct >= 80) return 'pv-usage-bar-fill--danger';
		if (pct >= 60) return 'pv-usage-bar-fill--warn';
		return '';
	}

	function storageKey(storage: string, node: string): string {
		return `${node}/${storage}`;
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

<!-- Gradient page header -->
<div class="pv-header -mx-6 -mt-6 mb-6">
	<div class="pv-header-flex">
		<div>
			<h1 class="pv-title">{$t('admin.storage.title')}</h1>
		</div>

		{#if !loading}
			<div class="flex items-center gap-3 flex-wrap">
				<div class="pv-header-stats">
					<div class="pv-header-stat">
						<div class="pv-header-stat-label">{$t('admin.storage.title')}</div>
						<div class="pv-header-stat-value">{filteredStorages.length}</div>
					</div>
					{#if enabledCount > 0}
						<div class="pv-header-stat">
							<div class="pv-header-stat-label">{$t('common.enabled')}</div>
							<div class="pv-header-stat-value">{enabledCount}</div>
						</div>
					{/if}
					{#if nodes.length > 1}
						<div class="pv-header-stat">
							<div class="pv-header-stat-label">{$t('nav.nodes')}</div>
							<div class="pv-header-stat-value">{nodes.length}</div>
						</div>
					{/if}
				</div>
				<Button
					class="pv-header-btn"
					variant="outline"
					size="sm"
					onclick={load}
					disabled={loading}
				>
					<ArrowsClockwise class="mr-1 h-4 w-4 {loading ? 'animate-spin' : ''}" />
					{$t('common.refresh')}
				</Button>
			</div>
		{/if}
	</div>
</div>

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
		<div class="pv-toolbar-info">
			{filteredStorages.length}
			{$t('admin.storage.title', { default: 'stockages' }).toLowerCase()}
			{#if selectedNode}· {selectedNode}{/if}
		</div>
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
	</div>

	<div class="pv-table-wrap">
		<table class="pv-table">
			<thead>
				<tr>
					<th>{$t('common.name')}</th>
					<th>{$t('common.type')}</th>
					<th>{$t('admin.storage.content')}</th>
					{#if nodes.length > 1 || selectedNode === ''}
						<th>{$t('common.node')}</th>
					{/if}
					<th class="pv-th-num">{$t('admin.storage.total')}</th>
					<th class="pv-th-num">{$t('admin.storage.used')}</th>
					<th>{$t('admin.storage.usage')}</th>
					<th class="text-center">{$t('common.enabled')}</th>
				</tr>
			</thead>
			<tbody>
				{#each filteredStorages as s}
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
{/if}
