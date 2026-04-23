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
	import {
		SlidersIcon,
		GlobeIcon,
		HardDriveIcon,
		InfoIcon,
		Cpu,
		Memory,
		Database,
		UsersThree,
		Camera,
		WifiHigh,
		CaretDown,
		FloppyDisk
	} from 'phosphor-svelte';
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
		toast.success($t('admin.limits.applyToAllNodes'));
	}

	// Collapsed state per node (node name -> collapsed boolean). Default: collapsed.
	let collapsedNodes = $state<Record<string, boolean>>({});

	function toggleNodeCollapsed(name: string): void {
		collapsedNodes = { ...collapsedNodes, [name]: !(collapsedNodes[name] ?? true) };
	}

	function isCollapsed(name: string): boolean {
		return collapsedNodes[name] ?? true;
	}

	function expandAllNodes(): void {
		const next: Record<string, boolean> = {};
		for (const n of sortedNodes) next[n] = false;
		collapsedNodes = next;
	}

	function collapseAllNodes(): void {
		const next: Record<string, boolean> = {};
		for (const n of sortedNodes) next[n] = true;
		collapsedNodes = next;
	}

	function rangeSummary(r: ResourceRange, unit: string = ''): string {
		return unit ? `${r.min}\u2013${r.max} ${unit}` : `${r.min}\u2013${r.max}`;
	}

	// Static class maps — Tailwind JIT can't resolve dynamic class names,
	// so we map each tint to the full class string.
	const TINT_CLASSES: Record<string, string> = {
		sky: 'bg-sky-50 text-sky-600 dark:bg-sky-950 dark:text-sky-400',
		violet: 'bg-violet-50 text-violet-600 dark:bg-violet-950 dark:text-violet-400',
		emerald: 'bg-emerald-50 text-emerald-600 dark:bg-emerald-950 dark:text-emerald-400'
	};

	function tintClasses(tint: string): string {
		return TINT_CLASSES[tint] ?? TINT_CLASSES.sky;
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
		<section class="pv-table-wrap">
			<header class="flex flex-wrap items-center justify-between gap-3 border-b border-border px-5 py-4">
				<div class="flex items-center gap-3">
					<div class="flex h-9 w-9 items-center justify-center rounded-lg bg-[hsl(var(--blaze-orange-50))] text-[hsl(var(--blaze-orange-700))] dark:bg-[hsl(var(--blaze-orange-900))] dark:text-[hsl(var(--blaze-orange-200))]">
						<SlidersIcon class="h-4 w-4" />
					</div>
					<div>
						<div class="flex items-center gap-2">
							<h2 class="text-sm font-semibold leading-none">{$t('admin.limits.vmResourceRanges')}</h2>
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
						<p class="mt-1 text-xs text-muted-foreground">{$t('admin.limits.vmResourceRangesTooltip')}</p>
					</div>
				</div>
				<Button size="sm" disabled={savingVm} onclick={() => saveSection((v) => (savingVm = v))}>
					<FloppyDisk class="mr-1 h-3.5 w-3.5" />
					{savingVm ? $t('common.saving') : $t('common.save')}
				</Button>
			</header>
			<div class="divide-y divide-border">
				{#each [
					{ key: 'sockets', icon: Cpu, label: $t('admin.limits.cpuSockets'), unit: '', tint: 'sky', range: limits.vm.sockets, bindMin: (v: number) => (limits!.vm.sockets.min = v), bindMax: (v: number) => (limits!.vm.sockets.max = v) },
					{ key: 'cores',   icon: Cpu, label: $t('admin.limits.cpuCores'),   unit: '', tint: 'sky', range: limits.vm.cores,   bindMin: (v: number) => (limits!.vm.cores.min = v),   bindMax: (v: number) => (limits!.vm.cores.max = v) },
					{ key: 'ram',     icon: Memory, label: $t('admin.limits.ramGb'),   unit: 'GB', tint: 'violet', range: limits.vm.ram,    bindMin: (v: number) => (limits!.vm.ram.min = v),     bindMax: (v: number) => (limits!.vm.ram.max = v) },
					{ key: 'disk',    icon: Database, label: $t('admin.limits.diskGb'), unit: 'GB', tint: 'emerald', range: limits.vm.disk, bindMin: (v: number) => (limits!.vm.disk.min = v),    bindMax: (v: number) => (limits!.vm.disk.max = v) }
				] as row}
					<div class="flex flex-wrap items-center gap-4 px-5 py-3">
						<div class="flex min-w-[180px] flex-1 items-center gap-3">
							<div class="flex h-8 w-8 items-center justify-center rounded-md {tintClasses(row.tint)}">
								<row.icon class="h-4 w-4" />
							</div>
							<div class="min-w-0">
								<div class="text-sm font-medium">{row.label}</div>
								<div class="text-xs text-muted-foreground tabular-nums">{rangeSummary(row.range, row.unit)}</div>
							</div>
						</div>
						<div class="flex items-center gap-3">
							<div class="flex flex-col items-stretch gap-1">
								<Label class="text-[10px] uppercase tracking-wider text-muted-foreground">{$t('admin.limits.min')}</Label>
								<Input type="number" min="1" class="h-9 w-20 text-center text-sm tabular-nums" value={row.range.min} oninput={(e) => row.bindMin(Number((e.target as HTMLInputElement).value))} />
							</div>
							<span class="mt-5 text-muted-foreground">→</span>
							<div class="flex flex-col items-stretch gap-1">
								<Label class="text-[10px] uppercase tracking-wider text-muted-foreground">{$t('admin.limits.max')}</Label>
								<Input type="number" min="1" class="h-9 w-20 text-center text-sm tabular-nums" value={row.range.max} oninput={(e) => row.bindMax(Number((e.target as HTMLInputElement).value))} />
							</div>
						</div>
					</div>
				{/each}
			</div>
		</section>

		<!-- Global Limits -->
		<section class="pv-table-wrap">
			<header class="flex flex-wrap items-center justify-between gap-3 border-b border-border px-5 py-4">
				<div class="flex items-center gap-3">
					<div class="flex h-9 w-9 items-center justify-center rounded-lg bg-[hsl(var(--blaze-orange-50))] text-[hsl(var(--blaze-orange-700))] dark:bg-[hsl(var(--blaze-orange-900))] dark:text-[hsl(var(--blaze-orange-200))]">
						<GlobeIcon class="h-4 w-4" />
					</div>
					<div>
						<div class="flex items-center gap-2">
							<h2 class="text-sm font-semibold leading-none">{$t('admin.limits.globalLimits')}</h2>
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
						<p class="mt-1 text-xs text-muted-foreground">{$t('admin.limits.globalLimitsTooltip')}</p>
					</div>
				</div>
				<Button size="sm" disabled={savingGlobal} onclick={() => saveSection((v) => (savingGlobal = v))}>
					<FloppyDisk class="mr-1 h-3.5 w-3.5" />
					{savingGlobal ? $t('common.saving') : $t('common.save')}
				</Button>
			</header>
			<div class="grid grid-cols-1 gap-3 p-5 sm:grid-cols-2 lg:grid-cols-4">
				<div class="rounded-lg border border-border bg-background p-4">
					<div class="mb-2 flex items-center gap-2 text-muted-foreground">
						<UsersThree class="h-4 w-4" />
						<Label class="text-xs font-medium">{$t('admin.limits.maxVmsPerUser')}</Label>
					</div>
					<Input type="number" min="0" class="text-sm tabular-nums" bind:value={limits.maxVmPerUser} />
				</div>
				<div class="rounded-lg border border-border bg-background p-4">
					<div class="mb-2 flex items-center gap-2 text-muted-foreground">
						<Camera class="h-4 w-4" />
						<Label class="text-xs font-medium">{$t('admin.limits.maxSnapshots')}</Label>
					</div>
					<Input type="number" min="0" class="text-sm tabular-nums" bind:value={limits.maxSnapshots} />
				</div>
				<div class="rounded-lg border border-border bg-background p-4">
					<div class="mb-2 flex items-center gap-2 text-muted-foreground">
						<WifiHigh class="h-4 w-4" />
						<Label class="text-xs font-medium">{$t('admin.limits.maxNetworkCards')}</Label>
					</div>
					<Input type="number" min="1" class="text-sm tabular-nums" bind:value={limits.maxNetworkCards} />
				</div>
				<div class="rounded-lg border border-border bg-background p-4">
					<div class="mb-2 flex items-center gap-2 text-muted-foreground">
						<Database class="h-4 w-4" />
						<Label class="text-xs font-medium">{$t('admin.limits.maxDisksPerVm')}</Label>
					</div>
					<Input type="number" min="1" class="text-sm tabular-nums" bind:value={limits.maxDiskPerVm} />
				</div>
			</div>
		</section>

		<!-- Node-Specific Limits -->
		{#if sortedNodes.length > 0}
			<section class="pv-table-wrap">
				<header class="flex flex-wrap items-center justify-between gap-3 border-b border-border px-5 py-4">
					<div class="flex items-center gap-3">
						<div class="flex h-9 w-9 items-center justify-center rounded-lg bg-[hsl(var(--blaze-orange-50))] text-[hsl(var(--blaze-orange-700))] dark:bg-[hsl(var(--blaze-orange-900))] dark:text-[hsl(var(--blaze-orange-200))]">
							<HardDriveIcon class="h-4 w-4" />
						</div>
						<div>
							<div class="flex items-center gap-2">
								<h2 class="text-sm font-semibold leading-none">{$t('admin.limits.nodeSpecificLimits')}</h2>
								<span class="rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground tabular-nums">
									{sortedNodes.length}
								</span>
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
							</div>
							<p class="mt-1 text-xs text-muted-foreground">{$t('admin.limits.nodeSpecificLimitsTooltip')}</p>
						</div>
					</div>
					<div class="flex flex-wrap items-center gap-2">
						<Button size="sm" variant="ghost" class="h-8 px-2 text-xs" onclick={expandAllNodes}>
							{$t('common.expandAll', { default: 'Expand all' })}
						</Button>
						<Button size="sm" variant="ghost" class="h-8 px-2 text-xs" onclick={collapseAllNodes}>
							{$t('common.collapseAll', { default: 'Collapse all' })}
						</Button>
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
							<FloppyDisk class="mr-1 h-3.5 w-3.5" />
							{savingNodes ? $t('common.saving') : $t('common.save')}
						</Button>
					</div>
				</header>

				<div class="divide-y divide-border">
					{#each visibleNodes as name}
						{@const status = nodeStatus(name)}
						{@const nodeLimits = limits.nodes[name]}
						{@const collapsed = isCollapsed(name)}
						<div>
							<!-- Node header (clickable to toggle) -->
							<button
								type="button"
								class="flex w-full flex-wrap items-center justify-between gap-3 px-5 py-3 text-left transition-colors hover:bg-muted/40"
								onclick={() => toggleNodeCollapsed(name)}
								aria-expanded={!collapsed}
							>
								<div class="flex items-center gap-3">
									<CaretDown class="h-4 w-4 text-muted-foreground transition-transform {collapsed ? '-rotate-90' : 'rotate-0'}" />
									<div class="pv-resource-icon pv-resource-icon--node">
										{name.slice(0, 2).toUpperCase()}
									</div>
									<div>
										<div class="flex items-center gap-2">
											<span class="pv-resource-name text-sm">{name}</span>
											<span class="pv-badge {status === 'online' ? 'pv-badge--online' : 'pv-badge--offline'}">
												{$t(`common.statusMap.${status}`, { default: status })}
											</span>
										</div>
										<div class="mt-0.5 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-muted-foreground tabular-nums">
											<span class="inline-flex items-center gap-1"><Cpu class="h-3 w-3" />{rangeSummary(nodeLimits.cores)}</span>
											<span class="inline-flex items-center gap-1"><Memory class="h-3 w-3" />{rangeSummary(nodeLimits.ram, 'GB')}</span>
											<span class="inline-flex items-center gap-1"><Database class="h-3 w-3" />{rangeSummary(nodeLimits.disk, 'GB')}</span>
										</div>
									</div>
								</div>
							</button>

							<!-- Collapsible content -->
							{#if !collapsed}
								<div class="divide-y divide-border border-t border-border bg-muted/20">
									{#each [
										{ icon: Cpu, label: $t('admin.limits.cpuSockets'), unit: '', tint: 'sky', range: nodeLimits.sockets, bindMin: (v: number) => (nodeLimits.sockets.min = v), bindMax: (v: number) => (nodeLimits.sockets.max = v) },
										{ icon: Cpu, label: $t('admin.limits.cpuCores'),   unit: '', tint: 'sky', range: nodeLimits.cores,   bindMin: (v: number) => (nodeLimits.cores.min = v),   bindMax: (v: number) => (nodeLimits.cores.max = v) },
										{ icon: Memory, label: $t('admin.limits.ramGb'),   unit: 'GB', tint: 'violet', range: nodeLimits.ram,  bindMin: (v: number) => (nodeLimits.ram.min = v),     bindMax: (v: number) => (nodeLimits.ram.max = v) },
										{ icon: Database, label: $t('admin.limits.diskGb'), unit: 'GB', tint: 'emerald', range: nodeLimits.disk, bindMin: (v: number) => (nodeLimits.disk.min = v),    bindMax: (v: number) => (nodeLimits.disk.max = v) }
									] as nrow}
										<div class="flex flex-wrap items-center gap-4 px-5 py-2.5 pl-14">
											<div class="flex min-w-[160px] flex-1 items-center gap-3">
												<div class="flex h-7 w-7 items-center justify-center rounded-md {tintClasses(nrow.tint)}">
													<nrow.icon class="h-3.5 w-3.5" />
												</div>
												<div class="text-xs font-medium">{nrow.label}</div>
											</div>
											<div class="flex items-center gap-2">
												<Input type="number" min="1" class="h-8 w-20 text-center text-xs tabular-nums" value={nrow.range.min} oninput={(e) => nrow.bindMin(Number((e.target as HTMLInputElement).value))} />
												<span class="text-xs text-muted-foreground">→</span>
												<Input type="number" min="1" class="h-8 w-20 text-center text-xs tabular-nums" value={nrow.range.max} oninput={(e) => nrow.bindMax(Number((e.target as HTMLInputElement).value))} />
												{#if nrow.unit}
													<span class="w-6 text-xs text-muted-foreground">{nrow.unit}</span>
												{:else}
													<span class="w-6"></span>
												{/if}
											</div>
										</div>
									{/each}
								</div>
							{/if}
						</div>
					{/each}
				</div>

				{#if totalNodePages > 1}
					<div class="flex items-center justify-between border-t border-border px-5 py-3">
						<span class="text-xs text-muted-foreground tabular-nums">
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
			</section>
		{/if}

	</div>
{/if}

</div>
