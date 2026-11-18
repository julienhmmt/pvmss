<script lang="ts">
	import type { BusyVM } from './dashboard.svelte';
	import { resolve } from '$app/paths';
	import { formatBytes } from '#lib/shared/format-bytes.js';
	import { m } from '#lib/paraglide/messages.js';

	let { topCPU, topMemory }: { topCPU: BusyVM[]; topMemory: BusyVM[] } = $props();

	const lists = $derived([
		{ key: 'cpu', title: m['admin.dashboard.busyCpuTitle'](), rows: topCPU, value: (vm: BusyVM) => `${vm.cpuPercent}%`, detail: () => '' },
		{
			key: 'memory',
			title: m['admin.dashboard.busyMemoryTitle'](),
			rows: topMemory,
			value: (vm: BusyVM) => `${vm.memoryPercent}%`,
			detail: (vm: BusyVM) => `${formatBytes(vm.memoryUsedBytes)} / ${formatBytes(vm.memoryTotalBytes)}`
		}
	]);
</script>

<div class="border-t border-border px-5 py-4" data-testid="dashboard-busy-vms">
	{#if topCPU.length === 0}
		<p class="text-sm text-muted-foreground" data-testid="dashboard-busy-none">{m['admin.dashboard.busyNone']()}</p>
	{:else}
		<div class="grid gap-6 md:grid-cols-2">
			{#each lists as list (list.key)}
				<section data-testid="dashboard-busy-{list.key}">
					<h3 class="mb-2 text-sm font-semibold text-foreground">{list.title}</h3>
					<ol class="space-y-1.5 text-sm">
						{#each list.rows as vm (`${vm.clusterKey}/${vm.vmid}`)}
							<li class="flex items-baseline gap-3">
								<div class="min-w-0 flex-1 truncate">
									<a class="font-medium text-foreground hover:underline" href={resolve('/admin/vms/[cluster]/[vmid]', { cluster: vm.clusterKey, vmid: String(vm.vmid) })}>{vm.name || vm.vmid}</a>
									<span class="ml-2 text-muted-foreground">{vm.cluster} · {vm.node}{vm.pool ? ` · ${vm.pool}` : ''}</span>
								</div>
								{#if list.detail(vm)}<span class="shrink-0 text-xs text-muted-foreground tabular-nums">{list.detail(vm)}</span>{/if}
								<span class="w-12 shrink-0 text-right font-mono font-semibold tabular-nums">{list.value(vm)}</span>
							</li>
						{/each}
					</ol>
				</section>
			{/each}
		</div>
	{/if}
</div>
