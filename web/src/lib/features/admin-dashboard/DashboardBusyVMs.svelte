<script lang="ts">
	import type { BusyVM } from './dashboard.svelte';
	import { resolve } from '$app/paths';
	import { formatBytes } from '#lib/shared/format-bytes.js';
	import { m } from '#lib/paraglide/messages.js';
	import { usageTone } from './dashboard-alerts';

	const TONE_BG = { success: 'bg-success', warning: 'bg-warning', destructive: 'bg-destructive' } as const;

	let { topCPU, topMemory }: { topCPU: BusyVM[]; topMemory: BusyVM[] } = $props();

	const lists = $derived([
		{ key: 'cpu', title: m['admin.dashboard.busyCpuTitle'](), rows: topCPU, value: (vm: BusyVM) => vm.cpuPercent, detail: () => '' },
		{
			key: 'memory',
			title: m['admin.dashboard.busyMemoryTitle'](),
			rows: topMemory,
			value: (vm: BusyVM) => vm.memoryPercent,
			detail: (vm: BusyVM) => `${formatBytes(vm.memoryUsedBytes)} / ${formatBytes(vm.memoryTotalBytes)}`
		}
	]);
</script>

<div class="border-t border-border px-5 py-4" data-testid="dashboard-busy-vms">
	{#if topCPU.length === 0}
		<p class="text-sm text-muted-foreground" data-testid="dashboard-busy-none">{m['admin.dashboard.busyNone']()}</p>
	{:else}
		<div class="grid gap-x-8 gap-y-6 md:grid-cols-2">
			{#each lists as list (list.key)}
				<section data-testid="dashboard-busy-{list.key}">
					<h3 class="mb-3 text-xs font-semibold tracking-wide text-muted-foreground uppercase">{list.title}</h3>
					<ol class="space-y-3">
						{#each list.rows as vm, i (`${vm.clusterKey}/${vm.vmid}`)}
							{@const pct = Math.min(100, list.value(vm))}
							<li class="grid grid-cols-[1.25rem_1fr_auto] items-center gap-x-3 gap-y-1">
								<span class="row-span-2 self-start font-mono text-xs text-muted-foreground tabular-nums">{i + 1}</span>
								<div class="min-w-0">
									<a class="block truncate text-sm font-medium text-foreground hover:underline" href={resolve('/admin/vms/[cluster]/[vmid]', { cluster: vm.clusterKey, vmid: String(vm.vmid) })}>{vm.name || vm.vmid}</a>
									<p class="truncate text-xs text-muted-foreground">{vm.node}{vm.pool ? ` · ${vm.pool}` : ''} · {vm.cluster}</p>
								</div>
								<div class="text-right">
									<p class="font-mono text-sm font-semibold tabular-nums">{list.value(vm)}%</p>
									{#if list.detail(vm)}<p class="text-xs text-muted-foreground tabular-nums">{list.detail(vm)}</p>{/if}
								</div>
								<div class="col-span-2 h-1 overflow-hidden rounded-full bg-muted">
									<div class="h-full rounded-full {TONE_BG[usageTone(pct)]}" style="width: {pct}%"></div>
								</div>
							</li>
						{/each}
					</ol>
				</section>
			{/each}
		</div>
	{/if}
</div>
