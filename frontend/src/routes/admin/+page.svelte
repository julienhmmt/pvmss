<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import LoadingToast from '$lib/components/data/LoadingToast.svelte';
	import PvHeader from '$lib/components/layout/PvHeader.svelte';
	import PvHeaderStat from '$lib/components/layout/PvHeaderStat.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import { Button } from '$lib/components/ui/button';
	import { getNodes } from '$lib/api/admin/nodes';
	import { getStorages } from '$lib/api/admin/storage';
	import { getAllVMs } from '$lib/api/admin/vms';
	import { getAppInfo } from '$lib/api/admin/appinfo';
	import { formatBytes, formatPercent } from '$lib/utils/format';
	import {
		ArrowsClockwise,
		HardDrives,
		Desktop,
		Database,
		CheckCircle,
		XCircle,
		GearSix,
		Info,
		GitBranch
	} from 'phosphor-svelte';
	import type { AppInfo, Node } from '$lib/types/admin';

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);

	let nodes = $state<Node[]>([]);
	let vmCount = $state(0);
	let storageTotal = $state(0);
	let storageUsed = $state(0);
	let storageCount = $state(0);
	let info = $state<AppInfo | null>(null);

	let nodesOnline = $derived(nodes.filter((n) => n.status === 'online').length);
	let nodesOffline = $derived(nodes.filter((n) => n.status !== 'online').length);
	let storageUsedPct = $derived(storageTotal > 0 ? Math.round((storageUsed / storageTotal) * 100) : 0);
	let envVarEntries = $derived(
		info?.envVars ? Object.entries(info.envVars).sort(([a], [b]) => a.localeCompare(b)) : []
	);

	function usageBarClass(pct: number) {
		if (pct >= 80) return 'pv-usage-bar-fill--danger';
		if (pct >= 60) return 'pv-usage-bar-fill--warn';
		return '';
	}

	async function load() {
		if (info !== null) {
			refreshing = true;
		} else {
			loading = true;
		}
		error = null;
		try {
			const [fetchedNodes, vms, storages, appInfo] = await Promise.all([
				getNodes(),
				getAllVMs(),
				getStorages(),
				getAppInfo()
			]);
			nodes = fetchedNodes;
			vmCount = vms.length;
			storageCount = storages.length;
			storageTotal = storages.reduce((s, x) => s + x.total, 0);
			storageUsed = storages.reduce((s, x) => s + x.used, 0);
			info = appInfo;
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	onMount(load);
</script>

<svelte:head>
	<title>PVMSS — {$t('admin.dashboard.title')}</title>
</svelte:head>

<PvHeader
	title={$t('admin.dashboard.title')}
	subtitle={info ? `PVMSS v${info.version} · ${info.environment}` : undefined}
	variant={info && !info.proxmoxConnected ? 'danger' : 'default'}
>
	{#snippet stats()}
		{#if !loading && info}
			<PvHeaderStat label={$t('nav.nodes')} value={nodes.length} />
			<PvHeaderStat label={$t('nav.vms')} value={vmCount} />
			<PvHeaderStat
				label="Proxmox"
				value={info.proxmoxConnected ? $t('admin.appinfo.connected') : $t('admin.appinfo.disconnected')}
				tone={info.proxmoxConnected ? 'default' : 'danger'}
			/>
		{/if}
	{/snippet}
	{#snippet actions()}
		{#if !loading}
			<Button class="pv-header-btn" variant="outline" size="sm" onclick={load} disabled={refreshing}>
				<ArrowsClockwise class="mr-1 h-4 w-4 {refreshing ? 'animate-spin' : ''}" />
				{$t('common.refresh')}
			</Button>
		{/if}
	{/snippet}
</PvHeader>

<div class="pv-content-width">

<LoadingToast visible={refreshing} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="card" rows={5} />
{:else if info}
	<div class="space-y-8">

		<!-- Cluster at a glance -->
		<section>
			<p class="pv-section-title">
				<HardDrives class="h-3.5 w-3.5" />
				{$t('admin.dashboard.title')}
			</p>
			<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">

				<!-- Nodes -->
				<div class="pv-table-wrap p-4 space-y-2">
					<div class="flex items-center gap-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
						<HardDrives class="h-3.5 w-3.5" />
						{$t('nav.nodes')}
					</div>
					<div class="text-3xl font-bold tabular-nums">{nodes.length}</div>
					<div class="flex gap-2 flex-wrap">
						{#if nodesOnline > 0}
							<span class="pv-badge pv-badge--online">{nodesOnline} {$t('common.statusMap.online')}</span>
						{/if}
						{#if nodesOffline > 0}
							<span class="pv-badge pv-badge--offline">{nodesOffline} {$t('common.statusMap.offline')}</span>
						{/if}
					</div>
				</div>

				<!-- VMs -->
				<div class="pv-table-wrap p-4 space-y-2">
					<div class="flex items-center gap-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
						<Desktop class="h-3.5 w-3.5" />
						{$t('nav.vms')}
					</div>
					<div class="text-3xl font-bold tabular-nums">{vmCount}</div>
					<div class="text-xs text-muted-foreground">{$t('common.total')}</div>
				</div>

				<!-- Storage -->
				<div class="pv-table-wrap p-4 space-y-2">
					<div class="flex items-center gap-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
						<Database class="h-3.5 w-3.5" />
						{$t('nav.storage')}
					</div>
					<div class="text-3xl font-bold tabular-nums">{storageCount}</div>
					{#if storageTotal > 0}
						<div class="space-y-1">
							<div class="pv-usage-bar">
								<div class="pv-usage-bar-track" style="flex:1">
									<div class="pv-usage-bar-fill {usageBarClass(storageUsedPct)}" style="width:{storageUsedPct}%"></div>
								</div>
								<span class="pv-usage-label">{storageUsedPct}%</span>
							</div>
							<div class="text-xs text-muted-foreground">
								{formatBytes(storageUsed)} / {formatBytes(storageTotal)}
							</div>
						</div>
					{/if}
				</div>

				<!-- Proxmox -->
				<div class="pv-table-wrap p-4 space-y-2">
					<div class="flex items-center gap-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
						<Info class="h-3.5 w-3.5" />
						Proxmox
					</div>
					<div class="flex items-center gap-2 mt-1">
						{#if info.proxmoxConnected}
							<span class="pv-badge pv-badge--online">{$t('admin.appinfo.connected')}</span>
						{:else}
							<span class="pv-badge pv-badge--offline">{$t('admin.appinfo.disconnected')}</span>
						{/if}
					</div>
					{#if info.clusterInfo?.isCluster}
						<div class="text-xs text-muted-foreground flex items-center gap-1">
							<GitBranch class="h-3 w-3" />
							{info.clusterInfo.clusterName} · {info.clusterInfo.nodeCount} {$t('nav.nodes').toLowerCase()}
						</div>
					{/if}
					<div class="pv-td-mono text-[0.65rem] truncate">{info.proxmoxUrl}</div>
				</div>
			</div>
		</section>

		<!-- Nodes detail -->
		{#if nodes.length > 0}
			<section>
				<p class="pv-section-title">
					<HardDrives class="h-3.5 w-3.5" />
					{$t('admin.nodes.title')}
				</p>
				<div class="pv-table-wrap">
					<table class="pv-table">
						<thead>
							<tr>
								<th>{$t('common.name')}</th>
								<th>{$t('common.status')}</th>
								<th>{$t('admin.nodes.cpu')}</th>
								<th>{$t('admin.nodes.ram')}</th>
								<th>{$t('admin.nodes.uptime')}</th>
							</tr>
						</thead>
						<tbody>
							{#each nodes as node, i (i)}
								{@const cpuPct = Math.round(node.cpu * 100)}
								{@const ramPct = Number(formatPercent(node.memory, node.maxMemory))}
								<tr class="pv-row">
									<td>
										<div class="pv-resource-cell">
											<div class="pv-resource-icon pv-resource-icon--node">{node.name.slice(0,2).toUpperCase()}</div>
											<span class="pv-resource-name">{node.name}</span>
										</div>
									</td>
									<td>
										<span class="pv-badge {node.status === 'online' ? 'pv-badge--online' : 'pv-badge--offline'}">
											{$t(`common.statusMap.${node.status}`, { default: node.status })}
										</span>
									</td>
									<td style="min-width:130px">
										<div class="pv-usage-bar">
											<div class="pv-usage-bar-track" style="flex:1">
												<div class="pv-usage-bar-fill {usageBarClass(cpuPct)}" style="width:{cpuPct}%"></div>
											</div>
											<span class="pv-usage-label">{cpuPct}%</span>
										</div>
									</td>
									<td style="min-width:130px">
										<div class="pv-usage-bar">
											<div class="pv-usage-bar-track" style="flex:1">
												<div class="pv-usage-bar-fill {usageBarClass(ramPct)}" style="width:{ramPct}%"></div>
											</div>
											<span class="pv-usage-label">{ramPct}%</span>
										</div>
									</td>
									<td class="pv-td-muted tabular-nums text-sm">{node.uptime > 0 ? Math.floor(node.uptime / 86400) + 'j ' + Math.floor((node.uptime % 86400) / 3600) + 'h' : '—'}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</section>
		{/if}

		<!-- PVMSS config -->
		<section>
			<p class="pv-section-title">
				<GearSix class="h-3.5 w-3.5" />
				{$t('admin.appinfo.title')}
			</p>
			<div class="pv-table-wrap">
				<table class="pv-table">
					<tbody>
						<tr class="pv-row">
							<th class="w-1/3 text-muted-foreground font-medium">{$t('admin.appinfo.version')}</th>
							<td><span class="pv-td-mono">{info.version}</span></td>
						</tr>
						<tr class="pv-row">
							<th class="text-muted-foreground font-medium">{$t('admin.appinfo.environment')}</th>
							<td><span class="pv-td-mono">{info.environment}</span></td>
						</tr>
						<tr class="pv-row">
							<th class="text-muted-foreground font-medium">{$t('admin.appinfo.goVersion')}</th>
							<td><span class="pv-td-mono">{info.goVersion}</span></td>
						</tr>
						<tr class="pv-row">
							<th class="text-muted-foreground font-medium">{$t('admin.appinfo.platform')}</th>
							<td><span class="pv-td-mono">{info.platform}</span></td>
						</tr>
						<tr class="pv-row">
							<th class="text-muted-foreground font-medium">{$t('admin.appinfo.offlineMode')}</th>
							<td>
								{#if info.offlineMode}
									<span class="pv-badge pv-badge--warn">{$t('common.yes')}</span>
								{:else}
									<span class="pv-badge pv-badge--online">{$t('common.no')}</span>
								{/if}
							</td>
						</tr>
					</tbody>
				</table>
			</div>
		</section>

		<!-- Env vars -->
		{#if envVarEntries.length > 0}
			<section>
				<p class="pv-section-title">
					<GearSix class="h-3.5 w-3.5" />
					{$t('admin.appinfo.envVars')}
				</p>
				<div class="pv-table-wrap">
					<table class="pv-table">
						<thead>
							<tr>
								<th>{$t('admin.appinfo.variable')}</th>
								<th>{$t('common.value')}</th>
							</tr>
						</thead>
						<tbody>
							{#each envVarEntries as [key, value], i (i)}
								<tr class="pv-row">
									<td><span class="pv-td-mono">{key}</span></td>
									<td class="pv-td-muted">{value}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</section>
		{/if}

	</div>
{/if}

</div>
