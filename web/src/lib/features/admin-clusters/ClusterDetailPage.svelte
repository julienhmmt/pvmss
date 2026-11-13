<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { afterNavigate, goto, pushState } from '$app/navigation';
	import { onMount } from 'svelte';
	import { onVisibleRefresh, STALE_REFRESH_MS } from '#lib/shared/visibility-refresh.js';
	import { ApiRequestError } from '#lib/shared/api/client.js';
	import { formatBytes, usagePercent } from '#lib/shared/format-bytes.js';
	import { formatDate, formatDateTime } from '#lib/shared/format-date.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import PageHeader from '#lib/shared/ui/PageHeader.svelte';
	import Pill from '#lib/shared/ui/Pill.svelte';
	import Skeleton from '#lib/shared/ui/Skeleton.svelte';
	import Tabs from '#lib/shared/ui/Tabs.svelte';
	import DashboardAttention from '#lib/features/admin-dashboard/DashboardAttention.svelte';
	import { usageTone } from '#lib/features/admin-dashboard/dashboard-alerts.js';
	import ClusterFormDialog from './ClusterFormDialog.svelte';
	import {
		AdminClustersStore,
		statusHint,
		statusLabel,
		statusTone,
		tlsAnnouncement,
		type ClusterInput
	} from './clusters.svelte';
	import {
		clusterTabParam,
		fetchClusterDetail,
		parseClusterTab,
		testClusterConnection,
		type ClusterDetail,
		type ClusterTab
	} from './cluster-detail.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		name: string;
	}

	let { name }: Props = $props();

	const TONE_BG = { success: 'bg-success', warning: 'bg-warning', destructive: 'bg-destructive' } as const;

	let detail = $state.raw<ClusterDetail | null>(null);
	let lastLoadedAt = 0;
	let loading = $state.raw(true);
	let error = $state.raw<string | null>(null);
	let announce = $state.raw<string | null>(null);
	let testing = $state.raw(false);
	let formOpen = $state.raw(false);
	const store = new AdminClustersStore();

	// The active tab is local state pushed into the URL so ?tab= deep links,
	// reloads and browser back/forward all land on the right panel.
	let activeTab: ClusterTab = $state(parseClusterTab(page.url.searchParams.get('tab')));

	const label = $derived(detail ? detail.displayName || detail.name : name);
	const inventory = $derived(detail?.inventory ?? null);
	const nodes = $derived(inventory?.nodes ?? []);
	const storages = $derived(inventory?.storages ?? []);
	const tls = $derived(detail?.tls ?? null);

	const tabs = $derived([
		{ id: 'status', label: () => m['admin.clusterDetail.tabStatus']() },
		{
			id: 'infrastructure',
			label: () => m['admin.clusterDetail.tabInfrastructure'](),
			...(inventory ? { count: nodes.length } : {})
		},
		{ id: 'connection', label: () => m['admin.clusters.connectionSection']() }
	]);

	afterNavigate(() => {
		activeTab = parseClusterTab(page.url.searchParams.get('tab'));
	});

	function selectTab(id: string): void {
		activeTab = parseClusterTab(id);
		const url: URL = new URL(page.url.href);
		const param: string | null = clusterTabParam(activeTab);
		if (param === null) url.searchParams.delete('tab');
		else url.searchParams.set('tab', param);
		pushState(`${url.pathname}${url.search}`, page.state);
	}

	$effect(() => {
		void load(name);
	});

	onMount(() =>
		onVisibleRefresh(() => {
			if (Date.now() - lastLoadedAt > STALE_REFRESH_MS) void load(name);
		})
	);

	async function load(target: string): Promise<void> {
		loading = true;
		error = null;
		try {
			detail = await fetchClusterDetail(target);
			lastLoadedAt = Date.now();
		} catch (err: unknown) {
			detail = null;
			error =
				err instanceof ApiRequestError && err.status === 404
					? m['admin.clusterDetail.notFound']()
					: m['admin.clusters.loadError']();
		} finally {
			loading = false;
		}
	}

	// The test endpoint answers 200 with the probe outcome either way, and the
	// server already folded the snapshot it gathered into the index - reloading
	// the detail picks up fresh counts and the persisted test result.
	async function runTest(): Promise<void> {
		testing = true;
		try {
			const result = await testClusterConnection(name);
			announce =
				tlsAnnouncement(label, result) ??
				m['admin.clusters.tested']({ name: label, status: result.status });
			detail = await fetchClusterDetail(name);
			lastLoadedAt = Date.now();
		} catch (err: unknown) {
			announce = err instanceof ApiRequestError ? err.message : m['admin.clusters.operationFailed']();
		} finally {
			testing = false;
		}
	}

	async function saveCluster(input: ClusterInput): Promise<void> {
		const succeeded = await store.update(name, {
			displayName: input.displayName ?? '',
			oidcRealm: input.oidcRealm ?? '',
			url: input.url,
			tlsInsecureSkipVerify: input.tlsInsecureSkipVerify,
			tokenId: input.tokenId,
			tokenSecret: input.tokenSecret,
			tls: input.tls
		});
		// Failure leaves the dialog open with store.error rendered inline.
		if (succeeded) {
			formOpen = false;
			announce = store.announce;
			detail = await fetchClusterDetail(name).catch(() => detail);
			lastLoadedAt = Date.now();
		}
	}

	async function renameCluster(newName: string): Promise<boolean> {
		const succeeded = await store.rename(name, newName);
		if (succeeded) {
			await goto(`${resolve('/admin/clusters/[name]', { name: newName })}?tab=connection`);
		}
		return succeeded;
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
		{#if detail}
			<Pill size="md" tone={statusTone(detail.lastTestStatus)} label={statusLabel(detail.lastTestStatus)} />
		{/if}
	{/snippet}
	{#snippet actions()}
		<Button variant="secondary" size="sm" {loading} onclick={() => void load(name)}>
			{loading ? m['common.refreshing']() : m['common.refresh']()}
		</Button>
	{/snippet}
</PageHeader>

{#if announce}<p class="sr-only" role="status" aria-live="polite">{announce}</p>{/if}

{#if detail === null}
	{#if loading}
		<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
		<div class="space-y-4"><Skeleton class="h-28 w-full" /><Skeleton class="h-52 w-full" /></div>
	{:else if error}
		<Alert>{error}</Alert>
	{/if}
{:else}
	<div data-testid="admin-cluster-detail">
		<Tabs {tabs} look="folder" bind:active={() => activeTab, selectTab} />

		<div id="panel-status" role="tabpanel" aria-labelledby="tab-status" hidden={activeTab !== 'status'} class="pv-tabpanel">
			<div class="space-y-6">
				{#if inventory?.stale}
					<Alert>{m['admin.clusterDetail.stale']()}</Alert>
				{/if}

				{#if inventory}
					<DashboardAttention alerts={inventory.alerts} />
				{/if}

				<Card pad="none" title={m['admin.clusterDetail.lastTest']()} titleId="cluster-test-section">
					<div class="flex flex-wrap items-center justify-between gap-3 p-5 text-sm">
						<div class="space-y-2">
							<div class="flex flex-wrap items-center gap-3">
								<Pill tone={statusTone(detail.lastTestStatus)} label={statusLabel(detail.lastTestStatus)} />
								{#if detail.lastTestAt}
									<time class="font-mono text-xs text-muted-foreground" datetime={detail.lastTestAt}>{formatDateTime(detail.lastTestAt)}</time>
								{/if}
							</div>
							{#if detail.lastTestMessage || statusHint(detail.lastTestStatus)}
								<p class="text-muted-foreground">{detail.lastTestMessage || statusHint(detail.lastTestStatus)}</p>
							{/if}
						</div>
						<Button variant="secondary" size="sm" loading={testing} label={m['admin.clusters.testLabel']({ name: label })} onclick={() => void runTest()}>
							{testing ? m['admin.clusters.testing']() : m['admin.clusters.test']()}
						</Button>
					</div>
				</Card>

				{#if inventory}
					<Card pad="none" title={m['admin.clusterDetail.inventory']()} titleId="cluster-inventory-section">
						<dl class="grid grid-cols-3 gap-5 p-5 text-sm">
							{@render field(m['admin.nodes.heading'](), String(nodes.length), true)}
							{@render field(m['admin.clusterDetail.vms'](), String(inventory.vmCount), true)}
							{@render field(m['admin.clusterDetail.vmsRunning'](), String(inventory.vmStatusCounts.running), true)}
						</dl>
						{#if inventory.refreshedAt}
							<p class="px-5 pb-4 text-xs text-muted-foreground">
								{m['admin.dashboard.refreshed']()}: <time class="font-mono" datetime={inventory.refreshedAt}>{formatDateTime(inventory.refreshedAt)}</time>
							</p>
						{/if}
					</Card>
				{/if}
			</div>
		</div>

		<div id="panel-infrastructure" role="tabpanel" aria-labelledby="tab-infrastructure" hidden={activeTab !== 'infrastructure'} class="pv-tabpanel">
			<div class="space-y-6">
				<Card pad="none" title={m['admin.nodes.heading']()} titleId="cluster-nodes-section">
					{#if nodes.length === 0}
						<p class="px-5 py-4 text-sm text-muted-foreground">{m['admin.nodes.emptyDescription']()}</p>
					{:else}
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
				</Card>

				<Card pad="none" title={m['admin.dashboard.storageTitle']()} titleId="cluster-storage-section">
					{#if storages.length === 0}
						<p class="px-5 py-4 text-sm text-muted-foreground">{m['admin.nodeDetails.storageEmpty']()}</p>
					{:else}
						<ul class="divide-y divide-border">
							{#each storages as storage (`${storage.node ?? ''}/${storage.name}`)}
								{@const percent = usagePercent(storage.usedBytes, storage.totalBytes)}
								<li class="space-y-2 px-5 py-3">
									<div class="flex flex-wrap items-center justify-between gap-2 text-sm"><span><span class="font-mono font-semibold">{storage.name}</span><span class="ml-2 text-xs text-muted-foreground">{storage.type}{#if storage.shared}&nbsp;· {m['admin.nodeDetails.shared']()}{:else if storage.node}&nbsp;· {storage.node}{/if}</span></span><span class="font-mono text-xs tabular-nums">{percent}%</span></div>
									{@render usageMeter(m['admin.dashboard.storageTitle'](), `${formatBytes(storage.usedBytes)} / ${formatBytes(storage.totalBytes)}`, percent)}
								</li>
							{/each}
						</ul>
					{/if}
				</Card>
			</div>
		</div>

		<div id="panel-connection" role="tabpanel" aria-labelledby="tab-connection" hidden={activeTab !== 'connection'} class="pv-tabpanel">
			<div class="space-y-6">
				<Card pad="none" title={m['admin.clusters.connectionSection']()} titleId="cluster-connection-section">
					{#snippet actions()}
						<Button variant="secondary" size="sm" label={m['admin.clusters.editLabel']({ name: label })} onclick={() => (formOpen = true)}>{m['common.edit']()}</Button>
					{/snippet}
					<dl class="grid gap-x-5 gap-y-4 p-5 text-sm sm:grid-cols-2 lg:grid-cols-3">
						{@render field(m['admin.clusterDetail.id'](), detail.name, true)}
						{@render field(m['admin.clusters.url'](), detail.url, true)}
						{@render field(m['admin.clusters.tokenId'](), detail.tokenId || ' - ', true)}
						{@render field(m['admin.clusters.tokenSecret'](), detail.tokenSet ? m['admin.clusterDetail.tokenSet']() : m['admin.clusterDetail.tokenMissing']())}
						{@render field(m['admin.clusterDetail.tls'](), detail.tlsInsecureSkipVerify ? m['admin.clusterDetail.tlsSkipped']() : m['admin.clusterDetail.tlsVerified']())}
						{@render field(m['admin.clusters.version'](), detail.proxmoxVersion ?? ' - ', true)}
					</dl>
				</Card>

				{#if tls}
					<Card pad="none" title={m['admin.clusters.tlsSection']()} titleId="cluster-tls-section">
						<dl class="grid gap-x-5 gap-y-4 p-5 text-sm sm:grid-cols-2 lg:grid-cols-3">
							{@render field(m['admin.clusterDetail.tlsCa'](), tls.caSet ? tls.caSubject ?? '' : m['admin.clusterDetail.tokenMissing'](), true)}
							{#if tls.caSet}
								{@render field(m['admin.clusterDetail.tlsCaExpires'](), tls.caNotAfter ? formatDate(tls.caNotAfter) : ' - ', true)}
							{/if}
							{@render field(m['admin.clusterDetail.tlsFingerprints'](), tls.nodeFingerprints.length > 0 ? String(tls.nodeFingerprints.length) : m['admin.clusterDetail.tokenMissing'](), true)}
							{@render field(m['admin.clusterDetail.tlsClientCert'](), tls.clientCertSet ? tls.clientCertSubject ?? '' : m['admin.clusterDetail.tokenMissing'](), true)}
							{#if tls.clientCertSet}
								{@render field(m['admin.clusterDetail.tlsCaExpires'](), tls.clientCertNotAfter ? formatDate(tls.clientCertNotAfter) : ' - ', true)}
							{/if}
						</dl>
					</Card>
				{/if}
			</div>
		</div>
	</div>

	<ClusterFormDialog
		bind:open={formOpen}
		editing={detail}
		saving={store.busy === `update:${name}` || store.busy === `rename:${name}`}
		error={store.error}
		onClose={() => (formOpen = false)}
		onSubmit={saveCluster}
		onRename={renameCluster}
	/>
{/if}

{#snippet field(term: string, value: string, mono = false)}
	<div><dt class="text-xs text-muted-foreground">{term}</dt><dd class="mt-1 break-words {mono ? 'font-mono' : ''}">{value}</dd></div>
{/snippet}

{#snippet usageMeter(label: string, value: string, percent: number)}
	<div class="space-y-1.5">
		<div class="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 text-xs"><span class="text-muted-foreground">{label}</span><span class="font-mono tabular-nums">{value}</span></div>
		<div role="progressbar" aria-label={label} aria-valuemin="0" aria-valuemax="100" aria-valuenow={percent} class="h-1.5 overflow-hidden rounded-full bg-muted"><div class="h-full rounded-full {TONE_BG[usageTone(percent)]} transition-[width] duration-500 motion-reduce:transition-none" style="width: {percent}%"></div></div>
	</div>
{/snippet}
