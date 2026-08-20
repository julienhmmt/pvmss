<script lang="ts">
	import type { AdminBridge } from './admin-catalog.svelte';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import SortableHeader from '$lib/shared/ui/SortableHeader.svelte';
	import SortableTooltipHeader from '$lib/shared/ui/SortableTooltipHeader.svelte';
	import { m } from '$lib/paraglide/messages.js';

	type BridgeSortColumn = 'name' | 'node' | 'comment' | 'enabled';

	interface Props {
		bridges: AdminBridge[];
		toggling: string | null;
		onToggle: (node: string, name: string, enabled: boolean) => void;
		sortBy: BridgeSortColumn;
		sortDir: 'asc' | 'desc';
		onSort: (column: BridgeSortColumn) => void;
	}

	let { bridges, toggling, onToggle, sortBy, sortDir, onSort }: Props = $props();

	function handleSort(column: string): void {
		onSort(column as BridgeSortColumn);
	}
</script>

<div class="overflow-x-auto rounded-lg border border-border">
	<table class="pv-responsive-table text-sm">
		<thead class="bg-muted/50 text-left">
			<tr>
				<SortableHeader text={m['common.name']()} column="name" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<SortableHeader text={m['common.node']()} column="node" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<SortableHeader text={m['admin.catalog.comment']()} column="comment" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<SortableTooltipHeader text={m['admin.catalog.statusColumn']()} tooltip={m['admin.catalog.tooltip.statusColumn']()} column="enabled" activeColumn={sortBy} {sortDir} onSort={handleSort} />
			</tr>
		</thead>
		<tbody>
			{#each bridges as bridge (bridge.name + bridge.node)}
				<tr class="border-t border-border">
					<td class="px-4 py-2 font-mono" data-label={m['common.name']()}>{bridge.name}</td>
					<td class="px-4 py-2 font-mono" data-label={m['common.node']()}>{bridge.node}</td>
					<td class="px-4 py-2" data-label={m['admin.catalog.comment']()}>{bridge.comment || '—'}</td>
					<td class="px-4 py-2" data-label={m['admin.catalog.statusColumn']()}>
						<span class="inline-flex items-center gap-2" aria-busy={toggling === `bridge:${bridge.node}/${bridge.name}`}>
							<Switch
								checked={bridge.enabled}
								label={bridge.enabled ? m['admin.catalog.revokeApproval']({ name: bridge.name }) : m['admin.catalog.approveName']({ name: bridge.name })}
								onToggle={() => onToggle(bridge.node, bridge.name, !bridge.enabled)}
							/>
							<span class="text-xs text-muted-foreground">
								{#if toggling === `bridge:${bridge.node}/${bridge.name}`}
									…
								{:else}
									{bridge.enabled ? m['admin.catalog.approvedStatus']() : m['admin.catalog.approveAction']()}
								{/if}
							</span>
						</span>
					</td>
				</tr>
			{:else}
				<tr><td colspan={4} class="p-0">
					<EmptyState title={m['admin.catalog.noBridges']()} />
				</td></tr>
			{/each}
		</tbody>
	</table>
</div>
