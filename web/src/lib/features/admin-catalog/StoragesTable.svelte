<script lang="ts">
	import type { AdminStorage } from './admin-catalog.svelte';
	import { formatBytes } from './format';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import NodeUsageBar from '$lib/shared/ui/NodeUsageBar.svelte';
	import TableHeader from '$lib/shared/ui/TableHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages.js';

	type StorageSortColumn = 'name' | 'node' | 'type' | 'usage' | 'enabled';

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
			<td class="px-4 py-3"><span class="sr-only">{m['admin.catalog.remove']()}</span></td>
		</tr>
	</thead>
	<tbody class="divide-y divide-border">
		{#each storages as storage (storage.name + storage.node)}
			<tr
				class="group transition-colors {storage.noStorage ? 'bg-muted/20 text-muted-foreground' : storage.missing ? 'opacity-60' : 'hover:bg-muted/40'}"
				data-testid="storage-row"
				data-storage-name={storage.noStorage ? '' : storage.name}
				data-storage-node={storage.node}
			>
				<td class="px-4 py-3.5" data-label={m['common.name']()}>
					{#if storage.noStorage}
						<span class="text-muted-foreground italic">{m['admin.storages.noAvailableStorage']()}</span>
					{:else}
						<span class="font-mono font-medium">{storage.name}</span>
						{#if storage.missing}
							<span
								class="ml-2 inline-flex items-center rounded-full border border-destructive/40 bg-destructive/10 px-2 py-0.5 text-xs font-medium text-destructive"
								data-testid="storage-missing-badge"
							>
								{m['admin.catalog.missingBadge']()}
							</span>
						{/if}
					{/if}
				</td>
				<td class="px-4 py-3.5 font-mono" data-label={m['common.node']()}>{storage.node}</td>
				<td class="px-4 py-3.5" data-label={m['common.type']()}>
					{#if !storage.noStorage}
						{storage.type}
					{/if}
				</td>
				<td class="px-4 py-3.5" data-label={m['admin.catalog.usage']()}>
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
				<td class="px-4 py-3.5" data-label={m['admin.catalog.statusColumn']()}>
					{#if !storage.noStorage}
						{#if storage.missing}
							<span class="text-xs text-muted-foreground">{m['admin.catalog.missingBadge']()}</span>
						{:else}
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
					{/if}
				</td>
				<td class="px-4 py-3.5" data-label={m['admin.catalog.remove']()}>
					{#if storage.missing}
						<Button
							variant="ghost"
							size="sm"
							onclick={() => onRemove(storage.name, storage.node)}
							data-testid="storage-remove"
						>
							{m['admin.catalog.remove']()}
						</Button>
					{/if}
				</td>
			</tr>
		{:else}
			<tr>
				<td colspan={6} class="p-0">
					<EmptyState title={m['admin.catalog.noStorages']()} />
				</td>
			</tr>
		{/each}
	</tbody>
</table>
