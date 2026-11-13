<script lang="ts">
	import { page } from '$app/state';
	import { onDestroy, onMount } from 'svelte';
	import { afterNavigate, pushState } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { get } from '#lib/shared/api/client.js';
	import { formatBytes, usagePercent } from '#lib/shared/format-bytes.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import PageHeader from '#lib/shared/ui/PageHeader.svelte';
	import Pill from '#lib/shared/ui/Pill.svelte';
	import Skeleton from '#lib/shared/ui/Skeleton.svelte';
	import Tabs from '#lib/shared/ui/Tabs.svelte';
	import MigrateDialog from '#lib/features/admin-migration/MigrateDialog.svelte';
	import { getTaskTrayContext } from '#lib/features/tasks/tasks.svelte.js';
	import { machineStatusView, type MachineStatusTone } from '#lib/features/vms/display-status.js';
	import { usageTone } from '#lib/features/admin-dashboard/dashboard-alerts.js';
	import {
		createHealthPoll,
		nodeTabParam,
		parseNodeTab,
		workloadStatusCounts,
		type NodeTab
	} from '#lib/features/admin-nodes/node-details.js';
	import { m } from '#lib/paraglide/messages.js';
	import { formatDateTime } from '#lib/shared/format-date.js';
	import Pagination from '#lib/shared/ui/Pagination.svelte';
	import { createPagination } from '#lib/shared/ui/list-pager.svelte.js';

	const TONE_BG = { success: 'bg-success', warning: 'bg-warning', destructive: 'bg-destructive' } as const;
	const SECONDS_PER_MINUTE = 60;
	const MINUTES_PER_HOUR = 60;
	const HOURS_PER_DAY = 24;
	const clusterKey: string = $derived(page.params.cluster ?? '');
	const nodeName: string = $derived(page.params.node ?? '');
	let detail = $state<NodeDetails | null>(null);
	let loading = $state<boolean>(true);
	let error = $state<string | null>(null);
	let requestedRoute: string = '';
	let requestId: number = 0;

	const ifacePager = createPagination('node-interfaces', () => detail?.network.interfaces ?? []);
	const vmsPager = createPagination('node-vms', () => detail?.inventory.vms ?? []);
	const storagePager = createPagination('node-storages', () => detail?.inventory.storages ?? []);
	const pciPager = createPagination('node-pci', () => pciDevices);

	interface NodeHealth {
		status: string;
		stale: boolean;
		cpuUsage: number;
		cpuModel: string;
		cpuCores: number;
		cpuTotalThreads: number;
		cpuSockets: number;
		loadAverage: string[];
		memoryTotalBytes: number;
		memoryUsedBytes: number;
		memoryFreeBytes: number;
		swapTotalBytes: number;
		swapUsedBytes: number;
		swapFreeBytes: number;
		rootfsTotalBytes: number;
		rootfsUsedBytes: number;
		rootfsFreeBytes: number;
		rootfsAvailableBytes: number;
		uptimeSeconds: number;
		proxmoxVersion: string;
		kernelVersion: string;
	}

	interface NodeNetworkInterface {
		name: string;
		type: string;
		active: boolean;
		address: string;
		cidr: string;
		gateway: string;
		address6: string;
		cidr6: string;
		gateway6: string;
		bridgePorts: string;
		bondSlaves: string;
		bondMode: string;
		vlanId: number;
		bridgeVlanAware: boolean;
		mtu: number;
	}

	interface PCIDevice {
		id: string;
		class: string;
		vendorId: string;
		vendorName: string;
		deviceId: string;
		deviceName: string;
		iommuGroup: number;
		mediatedDevice: boolean;
	}

	interface NodeVM {
		vmid: number;
		name: string;
		status: string;
		cpuCores: number;
		memoryTotalBytes: number;
		managed: boolean;
	}

	interface NodeStorage {
		name: string;
		type: string;
		shared: boolean;
		usedBytes: number;
		totalBytes: number;
	}

	interface NodeDetails {
		clusterKey: string;
		cluster: string;
		name: string;
		health: NodeHealth;
		network: { available: boolean; interfaces: NodeNetworkInterface[] };
		inventory: { refreshedAt: string; vms: NodeVM[]; storages: NodeStorage[] };
	}

	type PCIState = 'idle' | 'loading' | 'loaded' | 'unavailable';

	// The active tab is local state pushed into the URL so ?tab= deep links,
	// reloads and browser back/forward all land on the right panel.
	let activeTab: NodeTab = $state(parseNodeTab(page.url.searchParams.get('tab')));
	let pciOpen = $state<boolean>(false);
	let pciState = $state<PCIState>('idle');
	let pciDevices = $state<PCIDevice[]>([]);
	let migrating = $state<NodeVM | null>(null);

	const tabs = $derived([
		{ id: 'health', label: () => m['admin.nodeDetails.tabHealth']() },
		{
			id: 'workloads',
			label: () => m['admin.nodeDetails.workloadsTitle'](),
			...(detail ? { count: detail.inventory.vms.length } : {})
		},
		{ id: 'hardware', label: () => m['admin.nodeDetails.tabHardware']() }
	]);

	const workloadSummary: string = $derived.by(() => {
		const vms: NodeVM[] = detail?.inventory.vms ?? [];
		if (vms.length === 0) return m['admin.nodeDetails.vmsEmpty']();
		const parts: string[] = workloadStatusCounts(vms.map((vm) => vm.status)).map(
			([status, count]) => `${count} ${guestStatusLabel(status).toLowerCase()}`
		);
		return `${m['admin.nodeDetails.workloadVms']({ count: vms.length })} · ${parts.join(' · ')}`;
	});

	afterNavigate(() => {
		activeTab = parseNodeTab(page.url.searchParams.get('tab'));
	});

	function selectTab(id: string): void {
		activeTab = parseNodeTab(id);
		const url: URL = new URL(page.url.href);
		const param: string | null = nodeTabParam(activeTab);
		if (param === null) url.searchParams.delete('tab');
		else url.searchParams.set('tab', param);
		pushState(`${url.pathname}${url.search}`, page.state);
	}

	const healthPoll = createHealthPoll(() => void loadNode(clusterKey, nodeName));
	$effect(() => {
		if (activeTab === 'health') healthPoll.start();
		else healthPoll.stop();
	});
	onDestroy(() => healthPoll.stop());

	const stopListening = getTaskTrayContext().onTaskOk(() => void loadNode(clusterKey, nodeName));
	onDestroy(stopListening);

	onMount(() => {
		void loadNode(clusterKey, nodeName);
	});

	async function loadNode(cluster: string, node: string): Promise<void> {
		const currentRequest: number = ++requestId;
		const currentRoute: string = `${cluster}/${node}`;
		if (currentRoute !== requestedRoute) {
			detail = null;
			requestedRoute = currentRoute;
		}
		loading = true;
		error = null;
		try {
			const path: string = `/api/v1/admin/nodes/${encodeURIComponent(cluster)}/${encodeURIComponent(node)}`;
			const response: NodeDetails = await get<NodeDetails>(path);
			if (currentRequest === requestId) detail = response;
		} catch {
			if (currentRequest === requestId) error = m['admin.nodeDetails.loadError']();
		} finally {
			if (currentRequest === requestId) loading = false;
		}
	}

	function refreshNode(): void {
		void loadNode(clusterKey, nodeName);
		if (pciOpen && pciState !== 'loading') void loadPci();
	}

	async function loadPci(): Promise<void> {
		pciState = 'loading';
		try {
			const path: string = `/api/v1/admin/nodes/${encodeURIComponent(clusterKey)}/${encodeURIComponent(nodeName)}/pci`;
			const response: { available: boolean; devices: PCIDevice[] } = await get(path);
			pciDevices = response.devices ?? [];
			pciState = response.available ? 'loaded' : 'unavailable';
		} catch {
			pciState = 'unavailable';
		}
	}

	function onPciToggle(): void {
		if (pciOpen && pciState === 'idle') void loadPci();
	}

	function nodeStatusLabel(status: string): string {
		if (status === 'online') return m['admin.nodeDetails.online']();
		if (status === 'offline') return m['admin.nodeDetails.offline']();
		return m['admin.nodeDetails.unknown']();
	}

	function guestStatusLabel(status: string): string {
		if (status === 'running' || status === 'stopped' || status === 'paused') {
			return machineStatusView(status).label();
		}
		return status;
	}

	function guestStatusTone(status: string): MachineStatusTone {
		if (status === 'running' || status === 'stopped' || status === 'paused') {
			return machineStatusView(status).tone;
		}
		return 'off';
	}

	const backTarget: { href: string; label: string } = $derived.by(() => {
		const from: string | null = page.url.searchParams.get('from');
		if (from === 'dashboard') return { href: resolve('/admin'), label: m['admin.dashboard.title']() };
		if (from === 'policy') return { href: resolve('/admin/policy/nodes'), label: m['policy.nodeTitle']() };
		return { href: resolve('/admin/nodes'), label: m['admin.nodes.heading']() };
	});

	function formatUptime(seconds: number): string {
		const days: number = Math.floor(seconds / (SECONDS_PER_MINUTE * MINUTES_PER_HOUR * HOURS_PER_DAY));
		const hours: number = Math.floor((seconds / (SECONDS_PER_MINUTE * MINUTES_PER_HOUR)) % HOURS_PER_DAY);
		const minutes: number = Math.floor((seconds / SECONDS_PER_MINUTE) % MINUTES_PER_HOUR);
		return m['admin.nodeDetails.uptime']({ days, hours, minutes });
	}

	function interfaceAddresses(iface: NodeNetworkInterface): string {
		const addresses: string[] = [];
		if (iface.cidr || iface.address) addresses.push(iface.cidr || iface.address);
		if (iface.cidr6 || iface.address6) addresses.push(iface.cidr6 || iface.address6);
		return addresses.length > 0 ? addresses.join(' · ') : m['admin.nodeDetails.notReported']();
	}

	function interfaceGateways(iface: NodeNetworkInterface): string {
		const gateways: string[] = [];
		if (iface.gateway) gateways.push(iface.gateway);
		if (iface.gateway6) gateways.push(iface.gateway6);
		return gateways.length > 0 ? gateways.join(' · ') : m['admin.nodeDetails.notReported']();
	}

	function interfaceLinks(iface: NodeNetworkInterface): string {
		const links: string[] = [];
		if (iface.bridgePorts) links.push(`${m['admin.nodeDetails.bridgePorts']()}: ${iface.bridgePorts}`);
		if (iface.bondSlaves) links.push(`${m['admin.nodeDetails.bondSlaves']()}: ${iface.bondSlaves}`);
		if (iface.bondMode) links.push(`${m['admin.nodeDetails.bondMode']()}: ${iface.bondMode}`);
		return links.length > 0 ? links.join(' · ') : m['admin.nodeDetails.notReported']();
	}
</script>

<svelte:head>
	<title>{detail ? m['admin.nodeDetails.pageTitle']({ name: detail.name }) : m['admin.nodeDetails.title']()}</title>
</svelte:head>

<PageHeader
	title={detail?.name ?? m['admin.nodeDetails.title']()}
	eyebrow={detail?.cluster ?? m['admin.nodeDetails.eyebrow']()}
	description={m['admin.nodeDetails.description']()}
	back={{ href: backTarget.href, label: backTarget.label }}
	focusTarget
>
	{#snippet titleMeta()}
		{#if detail}
			<Pill
				size="md"
				tone={detail.health.status === 'online' ? 'ok' : detail.health.status === 'offline' ? 'error' : 'off'}
				label={nodeStatusLabel(detail.health.status)}
			/>
		{/if}
	{/snippet}
	{#snippet actions()}
		{#if detail}
			<span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
				{m['admin.nodeDetails.inventoryRefreshed']()}
				<time class="font-mono" datetime={detail.inventory.refreshedAt}>
					{detail.inventory.refreshedAt ? formatDateTime(detail.inventory.refreshedAt) : m['admin.nodeDetails.notReported']()}
				</time>
			</span>
		{/if}
		<Button variant="secondary" size="sm" loading={loading} onclick={refreshNode} data-testid="node-detail-refresh">
			{loading ? m['common.refreshing']() : m['common.refresh']()}
		</Button>
	{/snippet}
</PageHeader>

{#if detail === null}
	{#if loading}
		<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
		<div class="space-y-4" data-testid="node-detail-skeleton">
			<Skeleton class="h-28 w-full" />
			<Skeleton class="h-52 w-full" />
			<Skeleton class="h-52 w-full" />
		</div>
	{:else if error}
		<Alert>{error}</Alert>
	{/if}
{:else}
	{@const health = detail.health}
	{@const vms = detail.inventory.vms}
	<div data-testid="admin-node-details">
		{#if health.stale}
			<Alert tone="warning" role="status" class="mb-4" data-testid="node-health-stale">{m['admin.nodeDetails.healthStale']()}</Alert>
		{/if}

		<Tabs {tabs} look="folder" bind:active={() => activeTab, selectTab} />

		<div id="panel-health" role="tabpanel" aria-labelledby="tab-health" hidden={activeTab !== 'health'} class="pv-tabpanel">
			<div class="space-y-6">
				<Card pad="none" title={m['admin.nodeDetails.healthTitle']()} titleId="node-health-section">
					<div class="space-y-5 p-5">
						<dl class="flex flex-wrap gap-x-6 gap-y-1.5 text-xs">
							<div class="flex items-baseline gap-1.5">
								<dt class="text-muted-foreground">{m['admin.nodeDetails.uptimeLabel']()}</dt>
								<dd class="font-mono tabular-nums">{formatUptime(health.uptimeSeconds)}</dd>
							</div>
							<div class="flex items-baseline gap-1.5">
								<dt class="text-muted-foreground">{m['admin.nodeDetails.loadAverage']()}</dt>
								<dd class="font-mono tabular-nums">{health.loadAverage.join(' / ') || m['admin.nodeDetails.notReported']()}</dd>
							</div>
							<div class="flex items-baseline gap-1.5">
								<dt class="text-muted-foreground">{m['admin.nodeDetails.kernel']()}</dt>
								<dd class="break-words font-mono">{health.kernelVersion || m['admin.nodeDetails.notReported']()}</dd>
							</div>
							<div class="flex items-baseline gap-1.5">
								<dt class="text-muted-foreground">{m['admin.nodeDetails.proxmoxVersion']()}</dt>
								<dd class="font-mono">{health.proxmoxVersion || m['admin.nodeDetails.notReported']()}</dd>
							</div>
						</dl>
						<div class="grid gap-5 sm:grid-cols-2">
							{@render usageMeter(m['admin.nodeDetails.cpu'](), `${Math.round(health.cpuUsage * 100)}%`, Math.min(100, Math.round(health.cpuUsage * 100)))}
							{@render usageMeter(m['admin.nodeDetails.memory'](), `${formatBytes(health.memoryUsedBytes)} / ${formatBytes(health.memoryTotalBytes)}`, usagePercent(health.memoryUsedBytes, health.memoryTotalBytes))}
							{@render usageMeter(m['admin.nodeDetails.swap'](), `${formatBytes(health.swapUsedBytes)} / ${formatBytes(health.swapTotalBytes)}`, usagePercent(health.swapUsedBytes, health.swapTotalBytes))}
							{@render usageMeter(m['admin.nodeDetails.rootfs'](), `${formatBytes(health.rootfsUsedBytes)} / ${formatBytes(health.rootfsTotalBytes)}`, usagePercent(health.rootfsUsedBytes, health.rootfsTotalBytes))}
						</div>
						<div class="flex flex-wrap items-center justify-between gap-2 border-t border-border pt-4">
							<p class="text-sm text-muted-foreground" data-testid="node-workload-summary">{workloadSummary}</p>
							{#if vms.length > 0}
								<Button variant="secondary" size="sm" onclick={() => selectTab('workloads')} data-testid="node-view-workloads">
									{m['admin.nodeDetails.viewWorkloads']()}
								</Button>
							{/if}
						</div>
					</div>
				</Card>

				<Card pad="none" title={m['admin.nodeDetails.storageTitle']()} titleId="node-storage-section">
					{#if detail.inventory.storages.length === 0}
						<p class="px-5 py-4 text-sm text-muted-foreground">{m['admin.nodeDetails.storageEmpty']()}</p>
					{:else}
						<ul class="divide-y divide-border">
							{#each storagePager.rows as storage (`${storage.name}/${storage.type}`)}
								{@const percent = usagePercent(storage.usedBytes, storage.totalBytes)}
								<li class="space-y-2 px-5 py-3">
									<div class="flex flex-wrap items-center justify-between gap-2 text-sm"><span><span class="font-mono font-semibold">{storage.name}</span><span class="ml-2 text-xs text-muted-foreground">{storage.type}{#if storage.shared}&nbsp;· {m['admin.nodeDetails.shared']()}{/if}</span></span><span class="font-mono text-xs tabular-nums">{percent}%</span></div>
									{@render usageMeter(m['admin.nodeDetails.storageTitle'](), `${formatBytes(storage.usedBytes)} / ${formatBytes(storage.totalBytes)}`, percent)}
								</li>
							{/each}
						</ul>
						<Pagination pager={storagePager} />
					{/if}
				</Card>
			</div>
		</div>

		<div id="panel-workloads" role="tabpanel" aria-labelledby="tab-workloads" hidden={activeTab !== 'workloads'} class="pv-tabpanel">
			<Card pad="none" title={m['admin.nodeDetails.vmsTitle']()} titleId="node-vms-section">
				{#if vms.length === 0}
					<p class="px-5 py-4 text-sm text-muted-foreground">{m['admin.nodeDetails.vmsEmpty']()}</p>
				{:else}
					<div class="overflow-x-auto"><table class="pv-table pv-responsive-table">
						<caption class="sr-only">{m['admin.nodeDetails.vmsTitle']()}</caption>
						<thead><tr><th class="font-medium">{m['admin.nodeDetails.vmid']()}</th><th class="font-medium">{m['admin.nodeDetails.name']()}</th><th class="font-medium">{m['admin.nodeDetails.status']()}</th><th class="font-medium">{m['admin.nodeDetails.vcpu']()}</th><th class="text-right font-medium">{m['admin.nodeDetails.memory']()}</th><th class="text-right font-medium">{m['admin.nodeDetails.actions']()}</th></tr></thead>
						<tbody>{#each vmsPager.rows as vm (vm.vmid)}<tr><td class="font-mono" data-label={m['admin.nodeDetails.vmid']()}>{vm.vmid}</td><td data-label={m['admin.nodeDetails.name']()}>{vm.name || m['admin.nodeDetails.notReported']()}</td><td data-label={m['admin.nodeDetails.status']()}><Pill tone={guestStatusTone(vm.status)} label={guestStatusLabel(vm.status)} /></td><td class="font-mono tabular-nums" data-label={m['admin.nodeDetails.vcpu']()}>{vm.cpuCores}</td><td class="text-right font-mono tabular-nums" data-label={m['admin.nodeDetails.memory']()}>{formatBytes(vm.memoryTotalBytes)}</td><td class="text-right" data-label={m['admin.nodeDetails.actions']()}>{#if vm.managed}<Button variant="secondary" size="sm" label={m['admin.nodeDetails.migrateActionLabel']({ vmid: vm.vmid, name: vm.name })} onclick={() => (migrating = vm)} data-testid="node-vm-migrate-{vm.vmid}">{m['admin.nodeDetails.migrateAction']()}</Button>{/if}</td></tr>{/each}</tbody>
					</table></div>
					<Pagination pager={vmsPager} />
				{/if}
			</Card>
		</div>

		<div id="panel-hardware" role="tabpanel" aria-labelledby="tab-hardware" hidden={activeTab !== 'hardware'} class="pv-tabpanel">
			<div class="space-y-6">
				<Card pad="none" title={m['admin.nodeDetails.nodeInfoTitle']()} titleId="node-info-section">
					<dl class="grid grid-cols-2 gap-x-5 gap-y-4 p-5 text-sm">
						<div><dt class="text-xs text-muted-foreground">{m['admin.nodeDetails.cpuModel']()}</dt><dd class="mt-1 break-words">{health.cpuModel || m['admin.nodeDetails.notReported']()}</dd></div>
						<div><dt class="text-xs text-muted-foreground">{m['admin.nodeDetails.cores']()}</dt><dd class="mt-1 font-mono tabular-nums">{health.cpuCores}</dd></div>
						<div><dt class="text-xs text-muted-foreground">{m['admin.nodeDetails.threads']()}</dt><dd class="mt-1 font-mono tabular-nums">{health.cpuTotalThreads}</dd></div>
						<div><dt class="text-xs text-muted-foreground">{m['admin.nodeDetails.sockets']()}</dt><dd class="mt-1 font-mono tabular-nums">{health.cpuSockets}</dd></div>
					</dl>
				</Card>

				<Card pad="none" title={m['admin.nodeDetails.networkTitle']()} titleId="node-network-section">
					{#if !detail.network.available}
						<p role="status" class="px-5 py-4 text-sm text-muted-foreground">{m['admin.nodeDetails.sectionUnavailable']()}</p>
					{:else if detail.network.interfaces.length === 0}
						<p class="px-5 py-4 text-sm text-muted-foreground">{m['admin.nodeDetails.networkEmpty']()}</p>
					{:else}
						<div class="overflow-x-auto">
							<table class="pv-table pv-responsive-table">
								<caption class="sr-only">{m['admin.nodeDetails.networkTitle']()}</caption>
								<thead><tr>
									<th class="font-medium">{m['admin.nodeDetails.interface']()}</th><th class="font-medium">{m['admin.nodeDetails.type']()}</th><th class="font-medium">{m['admin.nodeDetails.state']()}</th><th class="font-medium">{m['admin.nodeDetails.address']()}</th><th class="font-medium">{m['admin.nodeDetails.gateway']()}</th><th class="font-medium">{m['admin.nodeDetails.topology']()}</th><th class="font-medium">{m['admin.nodeDetails.vlan']()}</th><th class="text-right font-medium">{m['admin.nodeDetails.mtu']()}</th>
								</tr></thead>
								<tbody>
									{#each ifacePager.rows as iface (iface.name)}
										<tr>
											<td class="font-mono font-medium" data-label={m['admin.nodeDetails.interface']()}>{iface.name}</td><td data-label={m['admin.nodeDetails.type']()}>{iface.type}</td><td data-label={m['admin.nodeDetails.state']()}><Pill tone={iface.active ? 'ok' : 'off'} label={iface.active ? m['admin.nodeDetails.active']() : m['admin.nodeDetails.inactive']()} /></td><td class="font-mono text-xs" data-label={m['admin.nodeDetails.address']()}>{interfaceAddresses(iface)}</td><td class="font-mono text-xs" data-label={m['admin.nodeDetails.gateway']()}>{interfaceGateways(iface)}</td><td class="max-w-64 text-xs text-muted-foreground" data-label={m['admin.nodeDetails.topology']()}>{interfaceLinks(iface)}</td><td class="font-mono tabular-nums" data-label={m['admin.nodeDetails.vlan']()}>{#if iface.vlanId > 0}{iface.vlanId}{:else if iface.bridgeVlanAware}{m['admin.nodeDetails.vlanAware']()}{:else}{m['admin.nodeDetails.notReported']()}{/if}</td><td class="text-right font-mono tabular-nums" data-label={m['admin.nodeDetails.mtu']()}>{iface.mtu || m['admin.nodeDetails.notReported']()}</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
						<Pagination pager={ifacePager} />
					{/if}
				</Card>

				<details class="group rounded-xl border border-border bg-card shadow-card" bind:open={pciOpen} ontoggle={onPciToggle} data-testid="node-pci-disclosure">
					<summary class="pv-focus flex cursor-pointer list-none items-center gap-3 rounded-xl px-5 py-4 [&::-webkit-details-marker]:hidden">
						<span class="flex-1 font-medium">{m['admin.nodeDetails.pciTitle']()}</span>
						<span class="w-4 text-center text-lg leading-none text-muted-foreground group-open:hidden" aria-hidden="true">+</span>
						<span class="hidden w-4 text-center text-lg leading-none text-muted-foreground group-open:inline" aria-hidden="true">&minus;</span>
					</summary>
					<div class="border-t border-border">
						{#if pciState === 'idle' || pciState === 'loading'}
							<div class="space-y-3 p-5" data-testid="node-pci-loading"><Skeleton class="h-4 w-full" /><Skeleton class="h-4 w-2/3" /></div>
						{:else if pciState === 'unavailable'}
							<p role="status" class="px-5 py-4 text-sm text-muted-foreground">{m['admin.nodeDetails.sectionUnavailable']()}</p>
						{:else if pciDevices.length === 0}
							<p class="px-5 py-4 text-sm text-muted-foreground">{m['admin.nodeDetails.pciEmpty']()}</p>
						{:else}
							<div class="overflow-x-auto">
								<table class="pv-table pv-responsive-table">
									<caption class="sr-only">{m['admin.nodeDetails.pciTitle']()}</caption>
									<thead><tr>
										<th class="font-medium">{m['admin.nodeDetails.pciId']()}</th><th class="font-medium">{m['admin.nodeDetails.device']()}</th><th class="font-medium">{m['admin.nodeDetails.vendor']()}</th><th class="font-medium">{m['admin.nodeDetails.class']()}</th><th class="font-medium">{m['admin.nodeDetails.iommuGroup']()}</th><th class="font-medium">{m['admin.nodeDetails.mediatedDevice']()}</th>
									</tr></thead>
									<tbody>
										{#each pciPager.rows as device (device.id)}
											<tr><td class="font-mono text-xs" data-label={m['admin.nodeDetails.pciId']()}>{device.id}</td><td data-label={m['admin.nodeDetails.device']()}>{device.deviceName || device.deviceId}</td><td data-label={m['admin.nodeDetails.vendor']()}>{device.vendorName || device.vendorId}</td><td class="font-mono text-xs" data-label={m['admin.nodeDetails.class']()}>{device.class}</td><td class="font-mono tabular-nums" data-label={m['admin.nodeDetails.iommuGroup']()}>{device.iommuGroup}</td><td data-label={m['admin.nodeDetails.mediatedDevice']()}>{device.mediatedDevice ? m['admin.nodeDetails.yes']() : m['admin.nodeDetails.no']()}</td></tr>
										{/each}
									</tbody>
								</table>
							</div>
							<Pagination pager={pciPager} />
						{/if}
					</div>
				</details>
			</div>
		</div>
	</div>
{/if}

{#if migrating !== null}
	<MigrateDialog cluster={clusterKey} vmid={migrating.vmid} name={migrating.name} onClose={() => (migrating = null)} />
{/if}

{#snippet usageMeter(label: string, value: string, percent: number)}
	<div class="space-y-1.5">
		<div class="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 text-xs"><span class="text-muted-foreground">{label}</span><span class="font-mono tabular-nums">{value}</span></div>
		<div role="progressbar" aria-label={label} aria-valuemin="0" aria-valuemax="100" aria-valuenow={percent} class="h-1.5 overflow-hidden rounded-full bg-muted"><div class="h-full rounded-full {TONE_BG[usageTone(percent)]} transition-[width] duration-500 motion-reduce:transition-none" style="width: {percent}%"></div></div>
	</div>
{/snippet}
