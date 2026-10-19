<script lang="ts">
	/**
	 * NodeLoadCell - the node's live, all-VMs load (versus the pvmss-tagged
	 * usage the capacity columns enforce). Three compact one-line meters -
	 * CPU, RAM, storage - plus the total VM count. An offline node renders a
	 * dash: zeros from the inventory would read as "idle", which is false.
	 */
	import type { NodeCapacity } from './policyNodes.svelte';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		node: NodeCapacity;
	}

	let { node }: Props = $props();

	function clamp(ratio: number): number {
		return Math.round(Math.min(1, Math.max(0, ratio)) * 100);
	}

	function barColor(pct: number): string {
		return pct >= 85 ? 'bg-destructive' : pct >= 60 ? 'bg-warning' : 'bg-success';
	}

	const memoryRatio = $derived(node.physicalRamGb > 0 ? node.nodeMemoryUsedGb / node.physicalRamGb : 0);
	const storageRatio = $derived(node.nodeStorageTotalGb > 0 ? node.nodeStorageUsedGb / node.nodeStorageTotalGb : 0);

	const meters = $derived([
		{ label: m['nodes.columnCpu'](), pct: clamp(node.nodeCpuUsage) },
		{ label: m['policy.colRam'](), pct: clamp(memoryRatio) },
		{ label: m['nodes.columnStorage'](), pct: clamp(storageRatio) }
	]);
</script>

{#if node.status === 'offline'}
	<span class="text-muted-foreground-subtle">—</span>
{:else}
	<div class="flex min-w-48 flex-col gap-1">
		{#each meters as meter (meter.label)}
			<div class="grid grid-cols-[4.5rem_1fr_2.5rem] items-center gap-2 text-xs">
				<span class="truncate text-muted-foreground">{meter.label}</span>
				<div
					class="h-1.5 overflow-hidden rounded-full bg-muted"
					role="meter"
					aria-valuemin={0}
					aria-valuemax={100}
					aria-valuenow={meter.pct}
					aria-label="{meter.label} {meter.pct}%"
				>
					<div class="h-full rounded-full {barColor(meter.pct)}" style="width: {meter.pct}%"></div>
				</div>
				<span class="text-right tabular-nums">{meter.pct}%</span>
			</div>
		{/each}
		<span class="mt-0.5 text-xs text-muted-foreground">{m['policy.totalVms']({ count: node.totalVms })}</span>
	</div>
{/if}
