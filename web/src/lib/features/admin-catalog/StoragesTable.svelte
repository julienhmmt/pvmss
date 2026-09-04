<script lang="ts">
	import type { AdminStorage } from './admin-catalog.svelte';
	import { formatBytes } from './format';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import NodeUsageBar from '$lib/shared/ui/NodeUsageBar.svelte';
	import TableHeader from '$lib/shared/ui/TableHeader.svelte';
	import { m } from '$lib/paraglide/messages.js';

	type StorageSortColumn = 'name' | 'node' | 'type' | 'usage' | 'enabled';

	interface Props {
		storages: AdminStorage[];
		toggling: string | null;
		onToggle: (name: string, node: string, enabled: boolean) => void;
		sortBy: StorageSortColumn;
		sortDir: 'asc' | 'desc';
		onSort: (column: StorageSortColumn) => void;
	}

	let { storages, toggling, onToggle, sortBy, sortDir, onSort }: Props = $props();

	function storageUsagePercent(used: number, total: number): number {
		if (total <= 0) return 0;
		return Math.min(100, Math.round((used / total) * 100));
	}

	function handleSort(column: string): void {
		onSort(column as StorageSortColumn);
	}
</script>

<table class="pv-responsive-table w-full text-sm">
	<caption class="sr-only">{m['admin.storages.heading']()}</caption>
	<thead class="bg-muted/60 text-left text-sm font-medium text-muted-foreground">
		<tr>
			<TableHeader text={m['common.name']()} column="name" activeColumn={sortBy} {sortDir} onSort={handleSort} />
			<TableHeader text={m['common.node']()} column="node" activeColumn={sortBy} {sortDir} onSort={handleSort} />
			<TableHeader
				text={m['common.type']()}
				tooltip={m['admin.catalog.tooltip.storageType']()}
				column="type"
				activeColumn={sortBy}
				{sortDir}
				onSort={handleSort}
			/>
			<TableHeader
				text={m['admin.catalog.usage']()}
				tooltip={m['admin.catalog.tooltip.storageUsage']()}
				column="usage"
				activeColumn={sortBy}
				{sortDir}
				onSort={handleSort}
			/>
			<TableHeader
				text={m['admin.catalog.statusColumn']()}
				tooltip={m['admin.catalog.tooltip.statusColumn']()}
				column="enabled"
				activeColumn={sortBy}
				{sortDir}
				onSort={handleSort}
			/>
		</tr>
	</thead>
	<tbody class="divide-y divide-border">
		{#each storages as storage (storage.name + storage.node)}
			<tr
				class="group transition-colors {storage.noStorage ? 'bg-muted/20 text-muted-foreground' : 'hover:bg-muted/40'}"
				data-testid="storage-row"
				data-storage-name={storage.noStorage ? '' : storage.name}
				data-storage-node={storage.node}
			>
				<td class="px-4 py-3.5" data-label={m['common.name']()}>
					{#if storage.noStorage}
						<span class="text-muted-foreground italic">{m['admin.storages.noAvailableStorage']()}</span>
					{:else}
						<span class="font-mono font-medium">{storage.name}</span>
					{/if}
				</td>
				<td class="px-4 py-3.5 font-mono" data-label={m['common.node']()}>{storage.node}</td>
				<td class="px-4 py-3.5" data-label={m['common.type']()}>
					{#if !storage.noStorage}
						{storage.type}
					{/if}
				</td>
				<td class="px-4 py-3.5" data-label={m['admin.catalog.usage']()}>
					{#if !storage.noStorage}
						<NodeUsageBar
							value={storage.totalBytes > 0 ? storage.usedBytes / storage.totalBytes : 0}
							label={m['admin.storages.usageLabel']({
								used: formatBytes(storage.usedBytes),
								total: formatBytes(storage.totalBytes),
								percent: storageUsagePercent(storage.usedBytes, storage.totalBytes)
							})}
						/>
					{/if}
				</td>
				<td class="px-4 py-3.5" data-label={m['admin.catalog.statusColumn']()}>
					{#if !storage.noStorage}
						<span
							class="inline-flex items-center gap-2"
							aria-busy={toggling === `storage:${storage.name}@${storage.node}`}
						>
							<Switch
								checked={storage.enabled}
								label={storage.enabled
									? m['admin.catalog.revokeApproval']({ name: storage.name })
									: m['admin.catalog.approveName']({ name: storage.name })}
								onToggle={() => onToggle(storage.name, storage.node, !storage.enabled)}
							/>
							<span class="text-xs text-muted-foreground">
								{#if toggling === `storage:${storage.name}@${storage.node}`}
									…
								{:else}
									{storage.enabled ? m['admin.catalog.approvedStatus']() : m['admin.catalog.approveAction']()}
								{/if}
							</span>
						</span>
					{/if}
				</td>
			</tr>
		{:else}
			<tr>
				<td colspan={5} class="p-0">
					<EmptyState title={m['admin.catalog.noStorages']()} />
				</td>
			</tr>
		{/each}
	</tbody>
</table>
