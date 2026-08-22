<script lang="ts">
	import type { AdminISO } from './admin-catalog.svelte';
	import { formatBytes } from './format';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import SortableHeader from '$lib/shared/ui/SortableHeader.svelte';
	import SortableTooltipHeader from '$lib/shared/ui/SortableTooltipHeader.svelte';
	import { m } from '$lib/paraglide/messages.js';

	type ISOSortColumn = 'file' | 'storage' | 'node' | 'size' | 'enabled';

	interface Props {
		isos: AdminISO[];
		toggling: string | null;
		onToggle: (node: string, storage: string, file: string, enabled: boolean) => void;
		sortBy: ISOSortColumn;
		sortDir: 'asc' | 'desc';
		onSort: (column: ISOSortColumn) => void;
	}

	let { isos, toggling, onToggle, sortBy, sortDir, onSort }: Props = $props();

	function handleSort(column: string): void {
		onSort(column as ISOSortColumn);
	}
</script>

<div class="overflow-x-auto rounded-lg border border-border">
	<table class="pv-responsive-table text-sm">
		<caption class="sr-only">{m['admin.isos.heading']()}</caption>
		<thead class="bg-muted/50 text-left">
			<tr>
				<SortableHeader text={m['admin.catalog.file']()} column="file" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<SortableHeader text={m['common.storage']()} column="storage" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<SortableHeader text={m['common.node']()} column="node" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<SortableHeader text={m['admin.catalog.size']()} column="size" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<SortableTooltipHeader
					text={m['admin.catalog.statusColumn']()}
					tooltip={m['admin.catalog.tooltip.statusColumn']()}
					column="enabled"
					activeColumn={sortBy}
					{sortDir}
					onSort={handleSort}
				/>
			</tr>
		</thead>
		<tbody>
			{#each isos as iso (iso.node + ':' + iso.storage + ':' + iso.file)}
				<tr class="border-t border-border" data-testid="iso-row">
					<td class="px-4 py-3 font-mono" data-label={m['admin.catalog.file']()}>{iso.file}</td>
					<td class="px-4 py-3 font-mono" data-label={m['common.storage']()}>{iso.storage}</td>
					<td class="px-4 py-3 font-mono" data-label={m['common.node']()}>{iso.node}</td>
					<td class="px-4 py-3" data-label={m['admin.catalog.size']()}>{formatBytes(iso.sizeBytes)}</td>
					<td class="px-4 py-3" data-label={m['admin.catalog.statusColumn']()}>
						<span
							class="inline-flex items-center gap-2"
							aria-busy={toggling === `iso:${iso.node}:${iso.storage}:${iso.file}`}
						>
							<Switch
								checked={iso.enabled}
								label={iso.enabled
									? m['admin.catalog.revokeApproval']({ name: iso.file })
									: m['admin.catalog.approveName']({ name: iso.file })}
								onToggle={() => onToggle(iso.node, iso.storage, iso.file, !iso.enabled)}
							/>
							<span class="text-xs text-muted-foreground">
								{#if toggling === `iso:${iso.node}:${iso.storage}:${iso.file}`}
									…
								{:else}
									{iso.enabled ? m['admin.catalog.approvedStatus']() : m['admin.catalog.approveAction']()}
								{/if}
							</span>
						</span>
					</td>
				</tr>
			{:else}
				<tr>
					<td colspan={5} class="p-0">
						<EmptyState title={m['admin.catalog.noIsos']()} />
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
