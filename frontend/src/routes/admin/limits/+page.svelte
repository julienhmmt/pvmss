<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import LoadingToast from '$lib/components/data/LoadingToast.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { getLimits, updateLimits } from '$lib/api/admin/limits';
	import { getNodes } from '$lib/api/admin/nodes';
	import { SlidersIcon, GlobeIcon, HardDriveIcon, InfoIcon } from 'phosphor-svelte';
	import { TooltipProvider, Tooltip, TooltipTrigger, TooltipContent } from '$lib/components/ui/tooltip';
	import { toast } from 'svelte-sonner';
	import type { Limits, ResourceRange, Node } from '$lib/types/admin';

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let savingVm = $state(false);
	let savingGlobal = $state(false);
	let savingNodes = $state(false);
	let limits = $state<Limits | null>(null);
	let allNodes = $state<Node[]>([]);

	// Ensure all known nodes always have an entry in limits.nodes
	function ensureAllNodes(lim: Limits, nodes: Node[]): Limits {
		const populated = { ...lim.nodes };
		for (const node of nodes) {
			if (!populated[node.name]) {
				populated[node.name] = {
					sockets: { min: lim.vm.sockets.min, max: lim.vm.sockets.max },
					cores: { min: lim.vm.cores.min, max: lim.vm.cores.max },
					ram: { min: lim.vm.ram.min, max: lim.vm.ram.max },
					disk: { min: lim.vm.disk.min, max: lim.vm.disk.max }
				};
			}
		}
		return { ...lim, nodes: populated };
	}

	async function load() {
		if (limits !== null) {
			refreshing = true;
		} else {
			loading = true;
		}
		error = null;
		try {
			const [lim, nodes] = await Promise.all([getLimits(), getNodes()]);
			allNodes = nodes;
			limits = ensureAllNodes(lim, nodes);
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	function coerceRange(r: ResourceRange): ResourceRange {
		return { min: Number(r.min), max: Number(r.max) };
	}

	function coerceLimits(l: Limits): Limits {
		const nodes: Record<string, { sockets: ResourceRange; cores: ResourceRange; ram: ResourceRange; disk: ResourceRange }> = {};
		for (const [k, v] of Object.entries(l.nodes)) {
			nodes[k] = {
				sockets: coerceRange(v.sockets),
				cores: coerceRange(v.cores),
				ram: coerceRange(v.ram),
				disk: coerceRange(v.disk)
			};
		}
		return {
			vm: {
				sockets: coerceRange(l.vm.sockets),
				cores: coerceRange(l.vm.cores),
				ram: coerceRange(l.vm.ram),
				disk: coerceRange(l.vm.disk)
			},
			nodes,
			maxSnapshots: Number(l.maxSnapshots),
			maxNetworkCards: Number(l.maxNetworkCards),
			maxDiskPerVm: Number(l.maxDiskPerVm),
			maxVmPerUser: Number(l.maxVmPerUser)
		};
	}

	async function saveSection(setSaving: (v: boolean) => void) {
		if (!limits) return;
		setSaving(true);
		try {
			await updateLimits(coerceLimits(limits));
			toast.success($t('admin.limits.saveSuccess'));
		} catch (e) {
			toast.error((e as Error).message);
		} finally {
			setSaving(false);
		}
	}

	const NODES_PER_PAGE = 6;
	let nodePage = $state(1);

	// Sorted node names: online nodes first, then offline
	let sortedNodes = $derived(
		allNodes
			.slice()
			.sort((a, b) => {
				if (a.status === b.status) return a.name.localeCompare(b.name);
				return a.status === 'online' ? -1 : 1;
			})
			.map((n) => n.name)
	);

	let totalNodePages = $derived(Math.ceil(sortedNodes.length / NODES_PER_PAGE));
	let visibleNodes = $derived(sortedNodes.slice((nodePage - 1) * NODES_PER_PAGE, nodePage * NODES_PER_PAGE));

	function nodeStatus(name: string): 'online' | 'offline' | 'unknown' {
		const n = allNodes.find((n) => n.name === name);
		if (!n) return 'unknown';
		return n.status === 'online' ? 'online' : 'offline';
	}

	function applyVmLimitsToAllNodes(): void {
		if (!limits) return;
		const updatedNodes: typeof limits.nodes = {};
		for (const name of Object.keys(limits.nodes)) {
			updatedNodes[name] = {
				sockets: { min: limits.vm.sockets.min, max: limits.vm.sockets.max },
				cores:   { min: limits.vm.cores.min,   max: limits.vm.cores.max },
				ram:     { min: limits.vm.ram.min,     max: limits.vm.ram.max },
				disk:    { min: limits.vm.disk.min,    max: limits.vm.disk.max }
			};
		}
		limits = { ...limits, nodes: updatedNodes };
	}

	onMount(load);
</script>

<svelte:head>
	<title>PVMSS — {$t('admin.limits.title')}</title>
</svelte:head>

<!-- Gradient page header -->
<div class="pv-header -mx-6 -mt-6 mb-6">
	<div class="pv-header-flex">
		<div>
			<p class="pv-eyebrow">{$t('nav.administration')}</p>
			<h1 class="pv-title">{$t('admin.limits.title')}</h1>
		</div>
		{#if !loading && limits}
			<div class="pv-header-stats">
				<div class="pv-header-stat">
					<div class="pv-header-stat-label">{$t('common.node')}</div>
					<div class="pv-header-stat-value">{allNodes.length}</div>
				</div>
				<div class="pv-header-stat">
					<div class="pv-header-stat-label">{$t('admin.limits.maxVmsPerUser')}</div>
					<div class="pv-header-stat-value">{limits.maxVmPerUser}</div>
				</div>
			</div>
		{/if}
	</div>
</div>

<div class="pv-content-width">

<LoadingToast visible={refreshing} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading || !limits}
	<LoadingSkeleton variant="form" rows={6} />
{:else}
	<div class="space-y-6">

		<!-- VM Resource Ranges -->
		<div class="rounded-lg border">
			<div class="flex items-center justify-between border-b px-4 py-3">
				<div class="flex items-center gap-2">
					<SlidersIcon class="h-4 w-4 text-muted-foreground" />
					<h2 class="text-sm font-semibold">{$t('admin.limits.vmResourceRanges')}</h2>
					<TooltipProvider>
						<Tooltip>
							<TooltipTrigger>
								<InfoIcon class="h-3.5 w-3.5 text-muted-foreground/60 cursor-help" />
							</TooltipTrigger>
							<TooltipContent>
								<p class="max-w-xs">{$t('admin.limits.vmResourceRangesTooltip')}</p>
							</TooltipContent>
						</Tooltip>
					</TooltipProvider>
				</div>
				<Button size="sm" disabled={savingVm} onclick={() => saveSection((v) => (savingVm = v))}>
					{savingVm ? $t('common.saving') : $t('common.save')}
				</Button>
			</div>
			<div class="p-4">
				<!-- Column headers -->
				<div class="mb-2 grid grid-cols-[1fr_80px_80px] gap-x-3 px-1">
					<div></div>
					<div class="text-center text-xs font-medium text-muted-foreground">{$t('admin.limits.min')}</div>
					<div class="text-center text-xs font-medium text-muted-foreground">{$t('admin.limits.max')}</div>
				</div>
				<div class="space-y-2">
					{#each [
						{ label: $t('admin.limits.cpuSockets'), min: limits.vm.sockets.min, max: limits.vm.sockets.max, bindMin: (v: number) => (limits!.vm.sockets.min = v), bindMax: (v: number) => (limits!.vm.sockets.max = v) },
						{ label: $t('admin.limits.cpuCores'),   min: limits.vm.cores.min,   max: limits.vm.cores.max,   bindMin: (v: number) => (limits!.vm.cores.min = v),   bindMax: (v: number) => (limits!.vm.cores.max = v) },
						{ label: $t('admin.limits.ramGb'),       min: limits.vm.ram.min,     max: limits.vm.ram.max,     bindMin: (v: number) => (limits!.vm.ram.min = v),     bindMax: (v: number) => (limits!.vm.ram.max = v) },
						{ label: $t('admin.limits.diskGb'),      min: limits.vm.disk.min,    max: limits.vm.disk.max,    bindMin: (v: number) => (limits!.vm.disk.min = v),    bindMax: (v: number) => (limits!.vm.disk.max = v) },
					] as row}
						<div class="grid grid-cols-[1fr_80px_80px] items-center gap-x-3">
							<Label class="text-xs">{row.label}</Label>
							<Input type="number" min="1" class="h-8 text-center text-sm" value={row.min} oninput={(e) => row.bindMin(Number((e.target as HTMLInputElement).value))} />
							<Input type="number" min="1" class="h-8 text-center text-sm" value={row.max} oninput={(e) => row.bindMax(Number((e.target as HTMLInputElement).value))} />
						</div>
					{/each}
				</div>
			</div>
		</div>

		<!-- Global Limits -->
		<div class="rounded-lg border">
			<div class="flex items-center justify-between border-b px-4 py-3">
				<div class="flex items-center gap-2">
					<GlobeIcon class="h-4 w-4 text-muted-foreground" />
					<h2 class="text-sm font-semibold">{$t('admin.limits.globalLimits')}</h2>
					<TooltipProvider>
						<Tooltip>
							<TooltipTrigger>
								<InfoIcon class="h-3.5 w-3.5 text-muted-foreground/60 cursor-help" />
							</TooltipTrigger>
							<TooltipContent>
								<p class="max-w-xs">{$t('admin.limits.globalLimitsTooltip')}</p>
							</TooltipContent>
						</Tooltip>
					</TooltipProvider>
				</div>
				<Button size="sm" disabled={savingGlobal} onclick={() => saveSection((v) => (savingGlobal = v))}>
					{savingGlobal ? $t('common.saving') : $t('common.save')}
				</Button>
			</div>
			<div class="p-4">
				<div class="grid grid-cols-2 gap-3">
					<div class="space-y-1.5">
						<Label class="text-xs">{$t('admin.limits.maxVmsPerUser')}</Label>
						<Input type="number" min="0" bind:value={limits.maxVmPerUser} />
					</div>
					<div class="space-y-1.5">
						<Label class="text-xs">{$t('admin.limits.maxSnapshots')}</Label>
						<Input type="number" min="0" bind:value={limits.maxSnapshots} />
					</div>
					<div class="space-y-1.5">
						<Label class="text-xs">{$t('admin.limits.maxNetworkCards')}</Label>
						<Input type="number" min="1" bind:value={limits.maxNetworkCards} />
					</div>
					<div class="space-y-1.5">
						<Label class="text-xs">{$t('admin.limits.maxDisksPerVm')}</Label>
						<Input type="number" min="1" bind:value={limits.maxDiskPerVm} />
					</div>
				</div>
			</div>
		</div>

		<!-- Node-Specific Limits -->
		{#if sortedNodes.length > 0}
			<div class="rounded-lg border">
				<div class="flex items-center justify-between border-b px-4 py-3">
					<div class="flex items-center gap-2">
						<HardDriveIcon class="h-4 w-4 text-muted-foreground" />
						<h2 class="text-sm font-semibold">{$t('admin.limits.nodeSpecificLimits')}</h2>
						<TooltipProvider>
							<Tooltip>
								<TooltipTrigger>
									<InfoIcon class="h-3.5 w-3.5 text-muted-foreground/60 cursor-help" />
								</TooltipTrigger>
								<TooltipContent>
									<p class="max-w-xs">{$t('admin.limits.nodeSpecificLimitsTooltip')}</p>
								</TooltipContent>
							</Tooltip>
						</TooltipProvider>
						<span class="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">
							{sortedNodes.length}
						</span>
					</div>
					<div class="flex items-center gap-2">
						<TooltipProvider>
							<Tooltip>
								<TooltipTrigger>
									<Button size="sm" variant="outline" onclick={applyVmLimitsToAllNodes} disabled={savingNodes}>
										{$t('admin.limits.applyToAllNodes')}
									</Button>
								</TooltipTrigger>
								<TooltipContent>
									<p class="max-w-xs">{$t('admin.limits.applyToAllNodesTooltip')}</p>
								</TooltipContent>
							</Tooltip>
						</TooltipProvider>
						<Button size="sm" disabled={savingNodes} onclick={() => saveSection((v) => (savingNodes = v))}>
							{savingNodes ? $t('common.saving') : $t('common.save')}
						</Button>
					</div>
				</div>

				<div class="divide-y">
					{#each visibleNodes as name}
						{@const status = nodeStatus(name)}
						{@const nodeLimits = limits.nodes[name]}
						<div class="px-4 py-3">
							<!-- Node header -->
							<div class="mb-3 flex items-center gap-2">
								<span class="h-2 w-2 rounded-full {status === 'online' ? 'bg-green-500' : 'bg-muted-foreground/40'}"></span>
								<span class="text-sm font-medium">{name}</span>
								<span class="text-xs text-muted-foreground">{$t(`common.statusMap.${status}`)}</span>
							</div>

							<!-- Resource grid -->
							<div class="grid grid-cols-[1fr_80px_80px] gap-x-3 gap-y-1.5">
								<!-- header row -->
								<div></div>
								<div class="text-center text-[11px] font-medium text-muted-foreground">{$t('admin.limits.min')}</div>
								<div class="text-center text-[11px] font-medium text-muted-foreground">{$t('admin.limits.max')}</div>

								<!-- Sockets -->
								<Label class="flex items-center text-xs">{$t('admin.limits.cpuSockets')}</Label>
								<Input type="number" min="1" class="h-7 text-center text-xs" bind:value={nodeLimits.sockets.min} />
								<Input type="number" min="1" class="h-7 text-center text-xs" bind:value={nodeLimits.sockets.max} />

								<!-- Cores -->
								<Label class="flex items-center text-xs">{$t('admin.limits.cpuCores')}</Label>
								<Input type="number" min="1" class="h-7 text-center text-xs" bind:value={nodeLimits.cores.min} />
								<Input type="number" min="1" class="h-7 text-center text-xs" bind:value={nodeLimits.cores.max} />

								<!-- RAM -->
								<Label class="flex items-center text-xs">{$t('admin.limits.ramGb')}</Label>
								<Input type="number" min="1" class="h-7 text-center text-xs" bind:value={nodeLimits.ram.min} />
								<Input type="number" min="1" class="h-7 text-center text-xs" bind:value={nodeLimits.ram.max} />

								<!-- Disk -->
								<Label class="flex items-center text-xs">{$t('admin.limits.diskGb')}</Label>
								<Input type="number" min="1" class="h-7 text-center text-xs" bind:value={nodeLimits.disk.min} />
								<Input type="number" min="1" class="h-7 text-center text-xs" bind:value={nodeLimits.disk.max} />
							</div>
						</div>
					{/each}
				</div>

				{#if totalNodePages > 1}
					<div class="flex items-center justify-between border-t px-4 py-2">
						<span class="text-xs text-muted-foreground">
							{$t('common.pageOf', { values: { page: nodePage, total: totalNodePages } })}
						</span>
						<div class="flex items-center gap-1">
							<Button
								size="sm"
								variant="ghost"
								class="h-7 px-2 text-xs"
								disabled={nodePage <= 1}
								onclick={() => nodePage--}
							>
								{$t('common.previous')}
							</Button>
							<Button
								size="sm"
								variant="ghost"
								class="h-7 px-2 text-xs"
								disabled={nodePage >= totalNodePages}
								onclick={() => nodePage++}
							>
								{$t('common.next')}
							</Button>
						</div>
					</div>
				{/if}
			</div>
		{/if}

	</div>
{/if}

</div>
