<script lang="ts">
	/**
	 * NodeLoadCell - the node's live, all-VMs load (versus the pvmss-tagged
	 * usage the capacity columns enforce). Three compact meters - CPU, RAM,
	 * storage - plus the total VM count. An offline node renders a dash:
	 * zeros from the inventory would read as "idle", which is false.
	 */
	import type { NodeCapacity } from './policyNodes.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import NodeUsageBar from '$lib/shared/ui/NodeUsageBar.svelte';

	interface Props {
		node: NodeCapacity;
	}

	let { node }: Props = $props();

	function percent(ratio: number): string {
		return `${Math.round(Math.min(1, Math.max(0, ratio)) * 100)}%`;
	}

	const memoryRatio = $derived(node.physicalRamGb > 0 ? node.nodeMemoryUsedGb / node.physicalRamGb : 0);
	const storageRatio = $derived(node.nodeStorageTotalGb > 0 ? node.nodeStorageUsedGb / node.nodeStorageTotalGb : 0);
</script>

{#if node.status === 'offline'}
	<span class="text-muted-foreground-subtle">—</span>
{:else}
	<div class="flex flex-col gap-1.5">
		<div class="flex flex-wrap gap-x-4 gap-y-1">
			<NodeUsageBar value={node.nodeCpuUsage} label="{m['nodes.columnCpu']()} {percent(node.nodeCpuUsage)}" />
			<NodeUsageBar value={memoryRatio} label="{m['policy.colRam']()} {percent(memoryRatio)}" />
			<NodeUsageBar value={storageRatio} label="{m['nodes.columnStorage']()} {percent(storageRatio)}" />
		</div>
		<span class="text-xs text-muted-foreground">{m['policy.totalVms']({ count: node.totalVms })}</span>
	</div>
{/if}
