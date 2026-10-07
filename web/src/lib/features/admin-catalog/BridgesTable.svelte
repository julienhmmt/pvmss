<script lang="ts">
	import type { AdminBridge, BridgeSortColumn } from './admin-catalog.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import StatusDot from '$lib/shared/ui/StatusDot.svelte';
	import TableHeader from '$lib/shared/ui/TableHeader.svelte';
	import ApprovalCell from './ApprovalCell.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		bridges: AdminBridge[];
		toggling: string | null;
		onToggle: (node: string, name: string, enabled: boolean) => void;
		onRemove: (node: string, name: string) => void;
		sortBy: BridgeSortColumn;
		sortDir: 'asc' | 'desc';
		onSort: (column: BridgeSortColumn) => void;
	}

	let { bridges, toggling, onToggle, onRemove, sortBy, sortDir, onSort }: Props = $props();

	type Tone = 'success' | 'destructive' | 'warning' | 'info' | 'muted';

	function bridgeActiveTone(active: boolean): Tone {
		return active ? 'success' : 'destructive';
	}

	function bridgeActiveLabel(active: boolean): string {
		return active ? m['common.active']() : m['common.inactive']();
	}
</script>

<table class="pv-table pv-responsive-table">
	<caption class="sr-only">{m['admin.bridges.heading']()}</caption>
	<thead>
		<tr>
			<TableHeader text={m['common.name']()} column="name" activeColumn={sortBy} {sortDir} {onSort} />
			<TableHeader text={m['common.node']()} column="node" activeColumn={sortBy} {sortDir} {onSort} />
			<TableHeader
				text={m['common.active']()}
				tooltip={m['admin.catalog.tooltip.bridgeActive']()}
				column="active"
				activeColumn={sortBy}
				{sortDir}
				{onSort}
			/>
			<TableHeader
				text={m['admin.catalog.comment']()}
				column="comment"
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
		{#each bridges as bridge (bridge.name + bridge.node)}
			<tr class="group transition-colors {bridge.missing ? 'opacity-60' : 'hover:bg-muted/40'}" data-testid="bridge-row">
				<td class="font-mono font-medium" data-label={m['common.name']()}>
					{bridge.name}{#if bridge.missing}
						<Pill
							tone="error"
							dot={false}
							label={m['admin.catalog.missingBadge']()}
							class="ml-2"
							data-testid="bridge-missing-badge"
						/>
					{/if}
				</td>
				<td class="font-mono" data-label={m['common.node']()}>{bridge.node}</td>
				<td data-label={m['common.active']()}>
					{#if bridge.missing}
						<span class="text-xs text-muted-foreground">{m['admin.catalog.missingBadge']()}</span>
					{:else}
						<StatusDot tone={bridgeActiveTone(bridge.active)} label={bridgeActiveLabel(bridge.active)} />
					{/if}
				</td>
				<td class="text-muted-foreground" data-label={m['admin.catalog.comment']()}>
					{bridge.comment || m['common.dash']()}
				</td>
				<td data-label={m['admin.catalog.statusColumn']()}>
					<ApprovalCell
						enabled={bridge.enabled}
						name={bridge.name}
						pending={toggling === `bridge:${bridge.node}/${bridge.name}`}
						missing={bridge.missing}
						onToggle={() => onToggle(bridge.node, bridge.name, !bridge.enabled)}
						onRemove={() => onRemove(bridge.node, bridge.name)}
						removeTestId="bridge-remove"
					/>
				</td>
			</tr>
		{:else}
			<tr>
				<td colspan={5} class="p-0">
					<EmptyState title={m['admin.catalog.noBridges']()} />
				</td>
			</tr>
		{/each}
	</tbody>
</table>
