<script lang="ts">
	import type { AdminStorage, StorageSortColumn } from './admin-catalog.svelte';
	import { formatBytes } from './format';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import NodeUsageBar from '$lib/shared/ui/NodeUsageBar.svelte';
	import TableHeader from '$lib/shared/ui/TableHeader.svelte';
	import ApprovalCell from './ApprovalCell.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		storages: AdminStorage[];
		toggling: string | null;
		onToggle: (name: string, node: string, enabled: boolean) => void;
		onRemove: (name: string, node: string) => void;
		sortBy: StorageSortColumn;
		sortDir: 'asc' | 'desc';
		onSort: (column: StorageSortColumn) => void;
	}

	let { storages, toggling, onToggle, onRemove, sortBy, sortDir, onSort }: Props = $props();

	function storageUsagePercent(used: number, total: number): number {
		if (total <= 0) return 0;
		return Math.min(100, Math.round((used / total) * 100));
	}
</script>

<table class="pv-table pv-responsive-table">
	<caption class="sr-only">{m['admin.storages.heading']()}</caption>
	<thead>
		<tr>
			<TableHeader text={m['common.name']()} column="name" activeColumn={sortBy} {sortDir} {onSort} />
			<TableHeader text={m['common.node']()} column="node" activeColumn={sortBy} {sortDir} {onSort} />
			<TableHeader
				text={m['common.type']()}
				tooltip={m['admin.catalog.tooltip.storageType']()}
				column="type"
				activeColumn={sortBy}
				{sortDir}
				{onSort}
			/>
			<TableHeader
				text={m['admin.catalog.usage']()}
				tooltip={m['admin.catalog.tooltip.storageUsage']()}
				column="usage"
				activeColumn={sortBy}
				{sortDir}
				{onSort}
			/>
			<TableHeader
				text={m['admin.catalog.statusColumn']()}
				tooltip={m['admin.catalog.tooltip.statusColumn']()}
				column="enabled"
				activeColumn={sortBy}
				{sortDir}
				{onSort}
			/>
		</tr>
	</thead>
	<tbody>
		{#each storages as storage (storage.name + storage.node)}
			<tr
				class="group transition-colors {storage.noStorage ? 'bg-muted/20 text-muted-foreground' : storage.missing ? 'opacity-60' : 'hover:bg-muted/40'}"
				data-testid="storage-row"
				data-storage-name={storage.noStorage ? '' : storage.name}
				data-storage-node={storage.node}
			>
				<td data-label={m['common.name']()}>
					{#if storage.noStorage}
						<span class="text-muted-foreground italic">{m['admin.storages.noAvailableStorage']()}</span>
					{:else}
						<span class="font-mono font-medium">{storage.name}</span>
						{#if storage.missing}
							<Pill
								tone="error"
								dot={false}
								label={m['admin.catalog.missingBadge']()}
								class="ml-2"
								data-testid="storage-missing-badge"
							/>
						{/if}
					{/if}
				</td>
				<td class="font-mono" data-label={m['common.node']()}>{storage.node}</td>
				<td data-label={m['common.type']()}>
					{#if !storage.noStorage}
						{storage.type}
					{/if}
				</td>
				<td data-label={m['admin.catalog.usage']()}>
					{#if !storage.noStorage && !storage.missing}
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
				<td data-label={m['admin.catalog.statusColumn']()}>
					{#if !storage.noStorage}
						<ApprovalCell
							enabled={storage.enabled}
							name={storage.name}
							pending={toggling === `storage:${storage.name}@${storage.node}`}
							missing={storage.missing}
							onToggle={() => onToggle(storage.name, storage.node, !storage.enabled)}
							onRemove={() => onRemove(storage.name, storage.node)}
							removeTestId="storage-remove"
						/>
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
