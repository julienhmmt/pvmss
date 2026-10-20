<script lang="ts">
	import { resolve } from '$app/paths';
	import { get } from '#lib/shared/api/client.js';
	import { formatBytes, usagePercent } from '#lib/shared/format-bytes.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import PageHeader from '#lib/shared/ui/PageHeader.svelte';
	import Pill from '#lib/shared/ui/Pill.svelte';
	import Skeleton from '#lib/shared/ui/Skeleton.svelte';
	import type { DashboardSummary } from '#lib/features/admin-dashboard/dashboard.svelte.js';
	import { statusHint, statusLabel, statusTone, type AdminCluster } from './clusters.svelte';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		name: string;
	}

	let { name }: Props = $props();
	let cluster = $state.raw<AdminCluster | null>(null);
	let summary = $state.raw<DashboardSummary | null>(null);
	let loading = $state.raw(true);
	let error = $state.raw<string | null>(null);

	// The dashboard labels storages by display name and nodes by key.
	const label = $derived(cluster ? cluster.displayName || cluster.name : name);
	const nodes = $derived(summary?.nodes.filter((n) => n.clusterKey === name) ?? []);
	const storages = $derived(summary?.storages.filter((s) => s.cluster === label) ?? []);
	const vmTotal = $derived(nodes.reduce((sum, n) => sum + n.vmCount, 0));
	const vmRunning = $derived(nodes.reduce((sum, n) => sum + n.vmRunningCount, 0));

	$effect(() => {
		void load(name);
	});

	async function load(target: string): Promise<void> {
		loading = true;
		error = null;
		try {
			const [clusters, dash] = await Promise.all([
				get<AdminCluster[]>('/api/v1/admin/clusters'),
				// Live inventory is optional: connection info still renders without it.
				get<DashboardSummary>('/api/v1/admin/dashboard').catch(() => null)
			]);
			cluster = clusters.find((c) => c.name === target) ?? null;
			summary = dash;
			if (cluster === null) error = m['admin.clusterDetail.notFound']();
		} catch {
			error = m['admin.clusters.loadError']();
		} finally {
			loading = false;
		}
	}
</script>

<PageHeader
	title={label}
	eyebrow={m['admin.clusterDetail.eyebrow']()}
	description={m['admin.clusterDetail.description']()}
	back={{ href: resolve('/admin/clusters'), label: m['admin.clusters.heading']() }}
	focusTarget
>
	{#snippet titleMeta()}
		{#if cluster}
			<Pill size="md" tone={statusTone(cluster.lastTestStatus)} label={statusLabel(cluster.lastTestStatus)} />
		{/if}
	{/snippet}
	{#snippet actions()}
		<Button variant="secondary" size="sm" {loading} onclick={() => void load(name)}>
			{loading ? m['common.refreshing']() : m['common.refresh']()}
		</Button>
	{/snippet}
</PageHeader>

{#if cluster === null}
	{#if loading}
		<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
		<div class="space-y-4"><Skeleton class="h-28 w-full" /><Skeleton class="h-52 w-full" /></div>
	{:else if error}
		<Alert>{error}</Alert>
	{/if}
{:else}
	<div class="space-y-6" data-testid="admin-cluster-detail">
		<Card pad="none" title={m['admin.clusters.connectionSection']()} titleId="cluster-connection-section">
			<dl class="grid gap-x-5 gap-y-4 p-5 text-sm sm:grid-cols-2 lg:grid-cols-3">
				{@render field(m['admin.clusterDetail.id'](), cluster.name, true)}
				{@render field(m['admin.clusters.url'](), cluster.url, true)}
				{@render field(m['admin.clusters.tokenId'](), cluster.tokenId || ' - ', true)}
				{@render field(m['admin.clusters.tokenSecret'](), cluster.tokenSet ? m['admin.clusterDetail.tokenSet']() : m['admin.clusterDetail.tokenMissing']())}
				{@render field(m['admin.clusterDetail.tls'](), cluster.tlsInsecureSkipVerify ? m['admin.clusterDetail.tlsSkipped']() : m['admin.clusterDetail.tlsVerified']())}
				{@render field(m['admin.clusters.version'](), cluster.proxmoxVersion ?? ' - ', true)}
			</dl>
		</Card>

		<Card pad="none" title={m['admin.clusterDetail.lastTest']()} titleId="cluster-test-section">
			<div class="space-y-2 p-5 text-sm">
				<div class="flex flex-wrap items-center gap-3">
					<Pill tone={statusTone(cluster.lastTestStatus)} label={statusLabel(cluster.lastTestStatus)} />
					{#if cluster.lastTestAt}
						<time class="font-mono text-xs text-muted-foreground" datetime={cluster.lastTestAt}>{new Date(cluster.lastTestAt).toLocaleString()}</time>
					{/if}
				</div>
				{#if cluster.lastTestMessage || statusHint(cluster.lastTestStatus)}
					<p class="text-muted-foreground">{cluster.lastTestMessage || statusHint(cluster.lastTestStatus)}</p>
				{/if}
			</div>
		</Card>

		<Card pad="none" title={m['admin.clusterDetail.inventory']()} titleId="cluster-inventory-section">
			{#if summary === null}
				<p role="status" class="px-5 py-4 text-sm text-muted-foreground">{m['admin.clusterDetail.inventoryUnavailable']()}</p>
			{:else}
				<dl class="grid grid-cols-3 gap-5 p-5 text-sm">
					{@render field(m['admin.nodes.heading'](), String(nodes.length), true)}
					{@render field(m['admin.clusterDetail.vms'](), String(vmTotal), true)}
					{@render field(m['admin.clusterDetail.vmsRunning'](), String(vmRunning), true)}
				</dl>
				{#if nodes.length > 0}
					<div class="overflow-x-auto">
						<table class="pv-table pv-responsive-table">
							<caption class="sr-only">{m['admin.nodes.heading']()}</caption>
							<thead><tr>
								<th class="font-medium">{m['common.name']()}</th><th class="font-medium">{m['common.status']()}</th><th class="font-medium">{m['admin.clusterDetail.vms']()}</th><th class="font-medium">{m['admin.nodeDetails.cpu']()}</th><th class="text-right font-medium">{m['admin.nodeDetails.memory']()}</th>
							</tr></thead>
							<tbody>
								{#each nodes as node (node.name)}
									<tr>
										<td class="font-mono font-medium" data-label={m['common.name']()}><a class="hover:underline" href={resolve('/admin/nodes/[cluster]/[node]', { cluster: name, node: node.name })}>{node.name}</a></td>
										<td data-label={m['common.status']()}><Pill tone={node.status === 'online' ? 'ok' : node.status === 'offline' ? 'error' : 'off'} label={node.status} /></td>
										<td class="font-mono tabular-nums" data-label={m['admin.clusterDetail.vms']()}>{node.vmRunningCount} / {node.vmCount}</td>
										<td class="font-mono tabular-nums" data-label={m['admin.nodeDetails.cpu']()}>{Math.round(node.cpuUsage * 100)}% · {node.cpuCores}</td>
										<td class="text-right font-mono tabular-nums" data-label={m['admin.nodeDetails.memory']()}>{formatBytes(node.memoryUsedBytes)} / {formatBytes(node.memoryTotalBytes)}</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			{/if}
		</Card>

		{#if summary !== null}
			<Card pad="none" title={m['admin.dashboard.storageTitle']()} titleId="cluster-storage-section">
				{#if storages.length === 0}
					<p class="px-5 py-4 text-sm text-muted-foreground">{m['admin.nodeDetails.storageEmpty']()}</p>
				{:else}
					<ul class="divide-y divide-border">
						{#each storages as storage (`${storage.node ?? ''}/${storage.name}`)}
							<li class="flex flex-wrap items-center justify-between gap-2 px-5 py-3 text-sm">
								<span><span class="font-mono font-semibold">{storage.name}</span><span class="ml-2 text-xs text-muted-foreground">{storage.type}{#if storage.shared} · {m['admin.nodeDetails.shared']()}{:else if storage.node} · {storage.node}{/if}</span></span>
								<span class="font-mono text-xs tabular-nums">{formatBytes(storage.usedBytes)} / {formatBytes(storage.totalBytes)} ({usagePercent(storage.usedBytes, storage.totalBytes)}%)</span>
							</li>
						{/each}
					</ul>
				{/if}
			</Card>
		{/if}
	</div>
{/if}

{#snippet field(term: string, value: string, mono = false)}
	<div><dt class="text-xs text-muted-foreground">{term}</dt><dd class="mt-1 break-words {mono ? 'font-mono' : ''}">{value}</dd></div>
{/snippet}
