<script lang="ts">
	import type { AdminStorage } from './admin-catalog.svelte';
	import { formatBytes } from './format';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import SortableHeader from '$lib/shared/ui/SortableHeader.svelte';
	import SortableTooltipHeader from '$lib/shared/ui/SortableTooltipHeader.svelte';
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

	function handleSort(column: string): void {
		onSort(column as StorageSortColumn);
	}
</script>

<div class="overflow-x-auto rounded-lg border border-border">
	<table class="pv-responsive-table text-sm">
		<thead class="bg-muted/50 text-left">
			<tr>
				<SortableHeader text={m['common.name']()} column="name" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<SortableHeader text={m['common.node']()} column="node" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<SortableTooltipHeader text={m['common.type']()} tooltip={m['admin.catalog.tooltip.storageType']()} column="type" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<SortableTooltipHeader text={m['admin.catalog.usage']()} tooltip={m['admin.catalog.tooltip.storageUsage']()} column="usage" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<SortableTooltipHeader text={m['admin.catalog.statusColumn']()} tooltip={m['admin.catalog.tooltip.statusColumn']()} column="enabled" activeColumn={sortBy} {sortDir} onSort={handleSort} />
			</tr>
		</thead>
		<tbody>
			{#each storages as storage (storage.name + storage.node)}
				<tr class="border-t border-border">
					<td class="px-4 py-2 font-mono" data-label={m['common.name']()}>{storage.name}</td>
					<td class="px-4 py-2 font-mono" data-label={m['common.node']()}>{storage.node}</td>
					<td class="px-4 py-2" data-label={m['common.type']()}>{storage.type}</td>
					<td class="px-4 py-2" data-label={m['admin.catalog.usage']()}>{formatBytes(storage.usedBytes)} / {formatBytes(storage.totalBytes)}</td>
					<td class="px-4 py-2" data-label={m['admin.catalog.statusColumn']()}>
						<span
							class="inline-flex items-center gap-2"
							aria-busy={toggling === `storage:${storage.name}@${storage.node}`}
						>
							<Switch
								checked={storage.enabled}
								label={storage.enabled ? m['admin.catalog.revokeApproval']({ name: storage.name }) : m['admin.catalog.approveName']({ name: storage.name })}
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
					</td>
				</tr>
			{:else}
				<tr><td colspan={5} class="p-0">
					<EmptyState title={m['admin.catalog.noStorages']()} />
				</td></tr>
			{/each}
		</tbody>
	</table>
</div>
