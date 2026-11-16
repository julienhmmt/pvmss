<script lang="ts">
	import CapacityCell from './CapacityCell.svelte';
	import NodeCapacityForm from './NodeCapacityForm.svelte';
	import NodeCapacityImportDialog from './NodeCapacityImportDialog.svelte';
	import NodeLoadCell from './NodeLoadCell.svelte';
	import { resolve } from '$app/paths';
	import {
		buildAllClustersNodeCapacityExport,
		buildNodeCapacityExport,
		cappedNodes,
		type NodeCapacity,
		type NodeCapacityExportSection,
		type NodeCapacityImportOutcome,
		type NodeCapacityPatch,
		type NodeCapacitySortColumn
	} from './policyNodes.svelte';
	import type { ClusterOption } from '#lib/shared/clusters.js';
	import type { NodeStatus } from '../cluster/nodes.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { formatDateTime } from '#lib/shared/format-date.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import PageHeader from '#lib/shared/ui/PageHeader.svelte';
	import ClusterSelector from '#lib/shared/ui/ClusterSelector.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Pill from '#lib/shared/ui/Pill.svelte';
	import TableCard from '#lib/shared/ui/TableCard.svelte';
	import Pagination from '#lib/shared/ui/Pagination.svelte';
	import { createPagination } from '#lib/shared/ui/list-pager.svelte.js';
	const pager = createPagination('policyNodes', () => nodes);
	import TableSkeleton from '#lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '#lib/shared/ui/EmptyState.svelte';
	import TableHeader from '#lib/shared/ui/TableHeader.svelte';

	interface Props {
		nodes: NodeCapacity[];
		loading: boolean;
		error: string | null;
		errorCode: string | null;
		saving: boolean;
		saveError: string | null;
		saveErrorCode: string | null;
		clusterOptions: ClusterOption[];
		cluster: string;
		onClusterChange: (value: string) => void;
		onLoad: () => void;
		onRetry: () => void;
		onSave: (node: string, patch: NodeCapacityPatch) => Promise<void>;
		refreshedAt: string | null;
		refreshing: boolean;
		refreshDisabled: boolean;
		refreshError: string | null;
		onRefresh: () => void;
		sortBy: NodeCapacitySortColumn;
		sortDir: 'asc' | 'desc';
		onSort: (column: NodeCapacitySortColumn) => void;
		onImport: (sections: NodeCapacityExportSection[], overwrite: boolean) => Promise<NodeCapacityImportOutcome[] | null>;
		onExportAll: () => Promise<NodeCapacityExportSection[] | null>;
	}

	let {
		nodes,
		loading,
		error,
		errorCode,
		saving,
		saveError,
		saveErrorCode,
		clusterOptions,
		cluster,
		onClusterChange,
		onLoad,
		onRetry,
		onSave,
		refreshedAt,
		refreshing,
		refreshDisabled,
		refreshError,
		onRefresh,
		sortBy,
		sortDir,
		onSort,
		onImport,
		onExportAll
	}: Props = $props();

	let expanded = $state<string | null>(null);
	let showImport = $state(false);
	let exportingAll = $state(false);

	// suffix names a single-node export: the header button exports every
	// capped node, a row button exports only its own.
	function downloadExport(items: NodeCapacity[], suffix = ''): void {
		const url = URL.createObjectURL(new Blob([buildNodeCapacityExport(cluster, items)], { type: 'application/json' }));
		const a = document.createElement('a');
		a.href = url;
		a.download = `pvmss-node-capacity-${cluster || 'cluster'}${suffix}.json`;
		a.click();
		setTimeout(() => URL.revokeObjectURL(url));
	}

	async function downloadExportAll(): Promise<void> {
		exportingAll = true;
		try {
			const sections = await onExportAll();
			if (sections === null) return;
			const url = URL.createObjectURL(new Blob([buildAllClustersNodeCapacityExport(sections)], { type: 'application/json' }));
			const a = document.createElement('a');
			a.href = url;
			a.download = 'pvmss-node-capacity-all-clusters.json';
			a.click();
			setTimeout(() => URL.revokeObjectURL(url));
		} finally {
			exportingAll = false;
		}
	}

	// The inline editor captures the caps when its row expands, so an import
	// closes it before the reload.
	async function handleImport(sections: NodeCapacityExportSection[], overwrite: boolean): Promise<NodeCapacityImportOutcome[] | null> {
		expanded = null;
		return onImport(sections, overwrite);
	}

	function toggleEditor(node: string): void {
		expanded = expanded === node ? null : node;
	}

	function handleSave(node: string, patch: NodeCapacityPatch): void {
		void onSave(node, patch)
			.then(() => (expanded = null))
			.catch(() => {});
	}

	function formatRefreshedAt(iso: string | null): string {
		if (!iso) return m['common.dash']();
		return formatDateTime(iso, 'short');
	}

	const statusTone: Record<NodeStatus, 'ok' | 'error' | 'off'> = {
		online: 'ok',
		offline: 'error',
		unknown: 'off'
	};

	function statusLabel(status: NodeStatus): string {
		switch (status) {
			case 'online':
				return m['admin.nodes.statusOnline']();
			case 'offline':
				return m['admin.nodes.statusOffline']();
			default:
				return m['admin.nodes.statusUnknown']();
		}
	}

	let refreshButtonLabel = $derived(
		refreshing ? m['nodes.refreshing']() : refreshDisabled ? m['nodes.refreshWait']() : m['nodes.refresh']()
	);
</script>

<svelte:head><title>{m['policy.nodeTitle']()} - PVMSS</title></svelte:head>

<PageHeader title={m['policy.nodeTitle']()} eyebrow={m['chrome.adminNav.policy']()} description={m['policy.nodeDescription']()} titleId="node-policy-title">
	{#snippet actions()}
		<div class="flex flex-wrap items-center gap-4">
			<ClusterSelector options={clusterOptions} value={cluster} onChange={onClusterChange} id="policy-nodes-cluster" />
			<Button variant="secondary" size="sm" disabled={nodes.length === 0} onclick={() => (showImport = true)}>{m['policy.nodesImport']()}</Button>
			<Button variant="secondary" size="sm" disabled={cappedNodes(nodes).length === 0} onclick={() => downloadExport(nodes)}>{m['policy.nodesExport']()}</Button>
			<Button variant="secondary" size="sm" disabled={exportingAll || clusterOptions.length === 0} onclick={() => void downloadExportAll()}>{m['policy.nodesExportAll']()}</Button>
			<div class="flex flex-col items-end gap-1">
				<Button variant="secondary" size="sm" onclick={onRefresh} disabled={refreshing || refreshDisabled} label={refreshButtonLabel} data-testid="refresh-button">
					{refreshButtonLabel}
				</Button>
				<p class="text-xs text-muted-foreground" data-testid="refreshed-at">
					{m['nodes.lastRefreshed']()} <time datetime={refreshedAt ?? undefined}>{formatRefreshedAt(refreshedAt)}</time>
				</p>
			</div>
		</div>
	{/snippet}
</PageHeader>

<section aria-labelledby="node-policy-title">
	{#if loading}
		<div role="status" aria-live="polite" class="sr-only">{m['policy.loading']()}</div>
		<TableSkeleton columns={7} />
	{:else if errorCode === 'inventory_not_ready'}
		<EmptyState
			title={m['policy.clusterUnreachableTitle']()}
			description={m['policy.clusterUnreachableDescription']()}
		>
			{#snippet actions()}
				<Button onclick={onRetry}>{m['policy.clusterUnreachableRetry']()}</Button>
			{/snippet}
		</EmptyState>
	{:else if error}
		<div class="space-y-3" role="alert"><p class="text-destructive">{error}</p><Button variant="secondary" onclick={onLoad}>{m['policy.retry']()}</Button></div>
	{:else}
		{#if refreshError}<Alert class="mb-4">{refreshError}</Alert>{/if}
		{#if saveError}<Alert class="mb-4">{saveError}</Alert>{/if}
		<TableCard>
			<table class="pv-table pv-responsive-table">
				<caption class="sr-only">{m['policy.nodeTitle']()}</caption>
				<thead>
					<tr>
						<TableHeader text={m['policy.node']()} column="node" activeColumn={sortBy} {sortDir} {onSort} />
						<TableHeader text={m['policy.colVms']()} column="vms" tooltip={m['policy.dimensionTooltip']()} activeColumn={sortBy} {sortDir} {onSort} />
						<TableHeader text={m['policy.colVcpus']()} column="vcpus" tooltip={m['policy.dimensionTooltip']()} activeColumn={sortBy} {sortDir} {onSort} />
						<TableHeader text={m['policy.colRam']()} column="ram" tooltip={m['policy.dimensionTooltip']()} activeColumn={sortBy} {sortDir} {onSort} />
						<TableHeader text={m['policy.colDisk']()} column="disk" tooltip={m['policy.dimensionTooltip']()} activeColumn={sortBy} {sortDir} {onSort} />
						<TableHeader text={m['policy.realLoad']()} column="load" tooltip={m['policy.realLoadTooltip']()} activeColumn={sortBy} {sortDir} {onSort} />
						<th scope="col" class="text-right font-medium">{m['policy.actions']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each pager.rows as node (node.node)}
						{@const isOpen = expanded === node.node}
						<!-- Offline is said by the status pill; dimming the row dragged every cell under AA. -->
						<tr class="group transition-colors hover:bg-muted/40">
							<th scope="row" class="text-left" data-label={m['policy.node']()}>
								<span class="flex flex-wrap items-center gap-x-2 gap-y-1">
									<a
										href="{resolve('/admin/nodes/[cluster]/[node]', { cluster, node: node.node })}?from=policy"
										class="pv-focus rounded-sm font-mono text-primary underline decoration-primary/40 underline-offset-2 hover:decoration-primary"
										data-testid="node-details-link"
									>
										{node.node}
									</a>
									<Pill tone={statusTone[node.status] ?? 'off'} label={statusLabel(node.status)} />
									{#if !node.approved}
										<span title={m['policy.nodeNotApprovedHint']()}>
											<Pill tone="warn" label={m['policy.nodeNotApproved']()} />
										</span>
									{/if}
								</span>
							</th>
							<td data-label={m['policy.colVms']()}><CapacityCell used={node.usedVms} cap={node.maxVms} /></td>
							<td data-label={m['policy.colVcpus']()}><CapacityCell used={node.usedVcpus} cap={node.maxVcpus} unit="vCPU" /></td>
							<td data-label={m['policy.colRam']()}><CapacityCell used={node.usedRamGb} cap={node.maxRamGb} unit="GB" /></td>
							<td data-label={m['policy.colDisk']()}><CapacityCell used={node.usedDiskGb} cap={node.maxDiskGb} unit="GB" /></td>
							<td data-label={m['policy.realLoad']()}><NodeLoadCell {node} /></td>
							<td data-label={m['policy.actions']()} class="text-right">
								<div class="flex items-center justify-end gap-2">
									<Button
										variant="secondary"
										size="sm"
										label={`${m['policy.nodesExport']()} ${node.node}`}
										disabled={cappedNodes([node]).length === 0}
										onclick={() => downloadExport([node], `-${node.node}`)}
									>
										{m['policy.nodesExport']()}
									</Button>
									<Button
										class="whitespace-nowrap"
										variant="secondary"
										size="sm"
										label={`${isOpen ? m['policy.closeEdit']() : m['policy.edit']()} ${node.node}`}
										aria-expanded={isOpen}
										onclick={() => toggleEditor(node.node)}
									>
										{isOpen ? m['policy.closeEdit']() : m['policy.edit']()}
									</Button>
								</div>
							</td>
						</tr>
						{#if isOpen}
							<tr class="bg-muted/30">
								<td colspan={7} data-nolabel="true" class="p-4">
									<NodeCapacityForm
										{node}
										{saving}
										error={saveError}
										errorCode={saveErrorCode}
										onClose={() => (expanded = null)}
										onSave={(patch) => handleSave(node.node, patch)}
									/>
								</td>
							</tr>
						{/if}
					{:else}
						<tr><td colspan={7} class="p-0">
							<EmptyState title={m['policy.noNodes']()} />
						</td></tr>
					{/each}
				</tbody>
			</table>
			{#snippet footer()}
				<Pagination {pager} />
			{/snippet}
		</TableCard>
	{/if}
</section>

<NodeCapacityImportDialog
	bind:open={showImport}
	{saving}
	{saveError}
	onImport={handleImport}
	onClose={() => (showImport = false)}
/>
