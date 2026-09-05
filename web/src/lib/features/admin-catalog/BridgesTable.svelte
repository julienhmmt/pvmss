<script lang="ts">
	import type { AdminBridge } from './admin-catalog.svelte';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import StatusDot from '$lib/shared/ui/StatusDot.svelte';
	import TableHeader from '$lib/shared/ui/TableHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages.js';

	type BridgeSortColumn = 'name' | 'node' | 'active' | 'comment' | 'enabled';

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

	function handleSort(column: string): void {
		onSort(column as BridgeSortColumn);
	}
</script>

<table class="pv-table pv-responsive-table">
	<caption class="sr-only">{m['admin.bridges.heading']()}</caption>
	<thead>
		<tr>
			<TableHeader text={m['common.name']()} column="name" activeColumn={sortBy} {sortDir} onSort={handleSort} />
			<TableHeader text={m['common.node']()} column="node" activeColumn={sortBy} {sortDir} onSort={handleSort} />
			<TableHeader
				text={m['common.active']()}
				tooltip={m['admin.catalog.tooltip.bridgeActive']()}
				column="active"
				activeColumn={sortBy}
				{sortDir}
				onSort={handleSort}
			/>
			<TableHeader
				text={m['admin.catalog.comment']()}
				column="comment"
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
			<td><span class="sr-only">{m['admin.catalog.remove']()}</span></td>
		</tr>
	</thead>
	<tbody>
		{#each bridges as bridge (bridge.name + bridge.node)}
			<tr class="group transition-colors {bridge.missing ? 'opacity-60' : 'hover:bg-muted/40'}" data-testid="bridge-row">
				<td class="font-mono font-medium" data-label={m['common.name']()}>
					{bridge.name}{#if bridge.missing}
						<span
							class="ml-2 inline-flex items-center rounded-full border border-destructive/40 bg-destructive/10 px-2 py-0.5 text-xs font-medium text-destructive"
							data-testid="bridge-missing-badge"
						>
							{m['admin.catalog.missingBadge']()}
						</span>
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
					{#if bridge.missing}
						<span class="text-xs text-muted-foreground">{m['admin.catalog.missingBadge']()}</span>
					{:else}
						<span
							class="inline-flex items-center gap-2"
							aria-busy={toggling === `bridge:${bridge.node}/${bridge.name}`}
						>
							<Switch
								checked={bridge.enabled}
								label={bridge.enabled
									? m['admin.catalog.revokeApproval']({ name: bridge.name })
									: m['admin.catalog.approveName']({ name: bridge.name })}
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
					{/if}
				</td>
				<td data-label={m['admin.catalog.remove']()}>
					{#if bridge.missing}
						<Button
							variant="ghost"
							size="sm"
							onclick={() => onRemove(bridge.node, bridge.name)}
							data-testid="bridge-remove"
						>
							{m['admin.catalog.remove']()}
						</Button>
					{/if}
				</td>
			</tr>
		{:else}
			<tr>
				<td colspan={6} class="p-0">
					<EmptyState title={m['admin.catalog.noBridges']()} />
				</td>
			</tr>
		{/each}
	</tbody>
</table>
