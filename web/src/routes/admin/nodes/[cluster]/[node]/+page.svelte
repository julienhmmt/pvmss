<script lang="ts">
	import { page } from '$app/state';
	import { afterNavigate } from '$app/navigation';
	import { onDestroy } from 'svelte';
	import { resolve } from '$app/paths';
	import { get } from '$lib/shared/api/client';
	import { formatBytes } from '$lib/shared/format-bytes';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';
	import MigrateDialog from '$lib/features/admin-migration/MigrateDialog.svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { usageTone } from '$lib/features/admin-dashboard/dashboard-alerts';
	import { m } from '$lib/paraglide/messages.js';

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

	interface LXCContainer {
		vmid: number;
		name: string;
		status: string;
		cpuCount: number;
		cpuUsage: number;
		memoryTotalBytes: number;
		memoryUsedBytes: number;
		diskTotalBytes: number;
		diskUsedBytes: number;
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
		pci: { available: boolean; devices: PCIDevice[] };
		containers: { available: boolean; containers: LXCContainer[] };
		inventory: { refreshedAt: string; vms: NodeVM[]; storages: NodeStorage[] };
	}

	let migrating = $state<NodeVM | null>(null);
	const stopListening = getTaskTrayContext().onTaskOk(() => void loadNode(clusterKey, nodeName));
	onDestroy(stopListening);

	afterNavigate(() => {
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
	}

	function usagePercent(used: number, total: number): number {
		if (total <= 0) return 0;
		return Math.min(100, Math.round((used / total) * 100));
	}

	function nodeStatusLabel(status: string): string {
		if (status === 'online') return m['admin.nodeDetails.online']();
		if (status === 'offline') return m['admin.nodeDetails.offline']();
		return m['admin.nodeDetails.unknown']();
	}

	function guestStatusLabel(status: string): string {
		if (status === 'running') return m['machine.status.running']();
		if (status === 'stopped') return m['machine.status.stopped']();
		if (status === 'paused') return m['admin.dashboard.vmPaused']();
		return status;
	}

	function uptimeLabel(seconds: number): string {
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
	back={{ href: resolve('/admin'), label: m['admin.dashboard.title']() }}
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
					{detail.inventory.refreshedAt ? new Date(detail.inventory.refreshedAt).toLocaleString() : m['admin.nodeDetails.notReported']()}
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
	<div class="space-y-6" data-testid="admin-node-details">
		{#if health.stale}
			<Alert tone="warning" role="status" data-testid="node-health-stale">{m['admin.nodeDetails.healthStale']()}</Alert>
		{/if}

		<Card pad="none" title={m['admin.nodeDetails.healthTitle']()} titleId="node-health-section">
			<div class="grid gap-5 p-5 lg:grid-cols-2">
				<div class="space-y-5">
					{@render usageMeter(m['admin.nodeDetails.cpu'](), `${Math.round(health.cpuUsage * 100)}%`, Math.min(100, Math.round(health.cpuUsage * 100)))}
					{@render usageMeter(m['admin.nodeDetails.memory'](), `${formatBytes(health.memoryUsedBytes)} / ${formatBytes(health.memoryTotalBytes)}`, usagePercent(health.memoryUsedBytes, health.memoryTotalBytes))}
					{@render usageMeter(m['admin.nodeDetails.swap'](), `${formatBytes(health.swapUsedBytes)} / ${formatBytes(health.swapTotalBytes)}`, usagePercent(health.swapUsedBytes, health.swapTotalBytes))}
					{@render usageMeter(m['admin.nodeDetails.rootfs'](), `${formatBytes(health.rootfsUsedBytes)} / ${formatBytes(health.rootfsTotalBytes)}`, usagePercent(health.rootfsUsedBytes, health.rootfsTotalBytes))}
				</div>
				<dl class="grid grid-cols-2 content-start gap-x-5 gap-y-4 text-sm">
					<div><dt class="text-xs text-muted-foreground">{m['admin.nodeDetails.cpuModel']()}</dt><dd class="mt-1 break-words">{health.cpuModel || m['admin.nodeDetails.notReported']()}</dd></div>
					<div><dt class="text-xs text-muted-foreground">{m['admin.nodeDetails.cores']()}</dt><dd class="mt-1 font-mono tabular-nums">{health.cpuCores}</dd></div>
					<div><dt class="text-xs text-muted-foreground">{m['admin.nodeDetails.threads']()}</dt><dd class="mt-1 font-mono tabular-nums">{health.cpuTotalThreads}</dd></div>
					<div><dt class="text-xs text-muted-foreground">{m['admin.nodeDetails.sockets']()}</dt><dd class="mt-1 font-mono tabular-nums">{health.cpuSockets}</dd></div>
					<div><dt class="text-xs text-muted-foreground">{m['admin.nodeDetails.loadAverage']()}</dt><dd class="mt-1 font-mono tabular-nums">{health.loadAverage.join(' / ') || m['admin.nodeDetails.notReported']()}</dd></div>
					<div><dt class="text-xs text-muted-foreground">{m['admin.nodeDetails.uptimeLabel']()}</dt><dd class="mt-1 font-mono tabular-nums">{uptimeLabel(health.uptimeSeconds)}</dd></div>
					<div><dt class="text-xs text-muted-foreground">{m['admin.nodeDetails.proxmoxVersion']()}</dt><dd class="mt-1 font-mono">{health.proxmoxVersion || m['admin.nodeDetails.notReported']()}</dd></div>
					<div><dt class="text-xs text-muted-foreground">{m['admin.nodeDetails.kernel']()}</dt><dd class="mt-1 break-words font-mono">{health.kernelVersion || m['admin.nodeDetails.notReported']()}</dd></div>
				</dl>
			</div>
		</Card>

		<Card pad="none" title={m['admin.nodeDetails.networkTitle']()} titleId="node-network-section">
			{#if !detail.network.available}
				<p role="status" class="px-5 py-4 text-sm text-muted-foreground">{m['admin.nodeDetails.sectionUnavailable']()}</p>
			{:else if detail.network.interfaces.length === 0}
				<p class="px-5 py-4 text-sm text-muted-foreground">{m['admin.nodeDetails.networkEmpty']()}</p>
			{:else}
				<div class="overflow-x-auto">
					<table class="w-full min-w-[760px] text-left text-sm">
						<thead class="bg-muted/50 text-xs text-muted-foreground"><tr>
							<th class="px-5 py-3 font-medium">{m['admin.nodeDetails.interface']()}</th><th class="px-3 py-3 font-medium">{m['admin.nodeDetails.type']()}</th><th class="px-3 py-3 font-medium">{m['admin.nodeDetails.state']()}</th><th class="px-3 py-3 font-medium">{m['admin.nodeDetails.address']()}</th><th class="px-3 py-3 font-medium">{m['admin.nodeDetails.gateway']()}</th><th class="px-3 py-3 font-medium">{m['admin.nodeDetails.topology']()}</th><th class="px-3 py-3 font-medium">{m['admin.nodeDetails.vlan']()}</th><th class="px-5 py-3 text-right font-medium">{m['admin.nodeDetails.mtu']()}</th>
						</tr></thead>
						<tbody class="divide-y divide-border">
							{#each detail.network.interfaces as iface (iface.name)}
								<tr>
									<td class="px-5 py-3 font-mono font-medium">{iface.name}</td><td class="px-3 py-3">{iface.type}</td><td class="px-3 py-3">{iface.active ? m['admin.nodeDetails.active']() : m['admin.nodeDetails.inactive']()}</td><td class="px-3 py-3 font-mono text-xs">{interfaceAddresses(iface)}</td><td class="px-3 py-3 font-mono text-xs">{interfaceGateways(iface)}</td><td class="max-w-64 px-3 py-3 text-xs text-muted-foreground">{interfaceLinks(iface)}</td><td class="px-3 py-3 font-mono tabular-nums">{#if iface.vlanId > 0}{iface.vlanId}{:else if iface.bridgeVlanAware}{m['admin.nodeDetails.vlanAware']()}{:else}{m['admin.nodeDetails.notReported']()}{/if}</td><td class="px-5 py-3 text-right font-mono tabular-nums">{iface.mtu || m['admin.nodeDetails.notReported']()}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</Card>

		<Card pad="none" title={m['admin.nodeDetails.pciTitle']()} titleId="node-pci-section">
			{#if !detail.pci.available}
				<p role="status" class="px-5 py-4 text-sm text-muted-foreground">{m['admin.nodeDetails.sectionUnavailable']()}</p>
			{:else if detail.pci.devices.length === 0}
				<p class="px-5 py-4 text-sm text-muted-foreground">{m['admin.nodeDetails.pciEmpty']()}</p>
			{:else}
				<div class="overflow-x-auto">
					<table class="w-full min-w-[700px] text-left text-sm">
						<thead class="bg-muted/50 text-xs text-muted-foreground"><tr>
							<th class="px-5 py-3 font-medium">{m['admin.nodeDetails.pciId']()}</th><th class="px-3 py-3 font-medium">{m['admin.nodeDetails.device']()}</th><th class="px-3 py-3 font-medium">{m['admin.nodeDetails.vendor']()}</th><th class="px-3 py-3 font-medium">{m['admin.nodeDetails.class']()}</th><th class="px-3 py-3 font-medium">{m['admin.nodeDetails.iommuGroup']()}</th><th class="px-5 py-3 font-medium">{m['admin.nodeDetails.mediatedDevice']()}</th>
						</tr></thead>
						<tbody class="divide-y divide-border">
							{#each detail.pci.devices as device (device.id)}
								<tr><td class="px-5 py-3 font-mono text-xs">{device.id}</td><td class="px-3 py-3">{device.deviceName || device.deviceId}</td><td class="px-3 py-3">{device.vendorName || device.vendorId}</td><td class="px-3 py-3 font-mono text-xs">{device.class}</td><td class="px-3 py-3 font-mono tabular-nums">{device.iommuGroup}</td><td class="px-5 py-3">{device.mediatedDevice ? m['admin.nodeDetails.yes']() : m['admin.nodeDetails.no']()}</td></tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</Card>

		<Card pad="none" title={m['admin.nodeDetails.workloadsTitle']()} titleId="node-workloads-section">
			<div class="space-y-6 p-5">
				<section aria-labelledby="node-vms-heading">
					<h3 id="node-vms-heading" class="mb-3 text-sm font-semibold">{m['admin.nodeDetails.vmsTitle']()}</h3>
					{#if detail.inventory.vms.length === 0}
						<p class="text-sm text-muted-foreground">{m['admin.nodeDetails.vmsEmpty']()}</p>
					{:else}
					<div class="overflow-x-auto"><table class="w-full min-w-[500px] text-left text-sm">
						<thead class="text-xs text-muted-foreground"><tr><th class="pb-2 font-medium">{m['admin.nodeDetails.vmid']()}</th><th class="pb-2 font-medium">{m['admin.nodeDetails.name']()}</th><th class="pb-2 font-medium">{m['admin.nodeDetails.status']()}</th><th class="pb-2 font-medium">{m['admin.nodeDetails.vcpu']()}</th><th class="pb-2 text-right font-medium">{m['admin.nodeDetails.memory']()}</th><th class="pb-2 text-right font-medium">{m['admin.nodeDetails.actions']()}</th></tr></thead>
						<tbody class="divide-y divide-border">{#each detail.inventory.vms as vm (vm.vmid)}<tr><td class="py-2 font-mono">{vm.vmid}</td><td class="py-2">{vm.name || m['admin.nodeDetails.notReported']()}</td><td class="py-2">{guestStatusLabel(vm.status)}</td><td class="py-2 font-mono tabular-nums">{vm.cpuCores}</td><td class="py-2 text-right font-mono tabular-nums">{formatBytes(vm.memoryTotalBytes)}</td><td class="py-2 text-right">{#if vm.managed}<Button variant="secondary" size="sm" label={m['admin.nodeDetails.migrateActionLabel']({ vmid: vm.vmid, name: vm.name })} onclick={() => (migrating = vm)} data-testid="node-vm-migrate-{vm.vmid}">{m['admin.nodeDetails.migrateAction']()}</Button>{/if}</td></tr>{/each}</tbody>
					</table></div>
					{/if}
				</section>
				<section aria-labelledby="node-lxc-heading">
					<div class="mb-3 flex items-center gap-2"><h3 id="node-lxc-heading" class="text-sm font-semibold">{m['admin.nodeDetails.lxcTitle']()}</h3>{#if !detail.containers.available}<span class="text-xs text-muted-foreground">{m['admin.nodeDetails.sectionUnavailable']()}</span>{/if}</div>
					{#if detail.containers.available && detail.containers.containers.length === 0}
						<p class="text-sm text-muted-foreground">{m['admin.nodeDetails.containersEmpty']()}</p>
					{:else if detail.containers.available}
						<div class="overflow-x-auto"><table class="w-full min-w-[600px] text-left text-sm">
							<thead class="text-xs text-muted-foreground"><tr><th class="pb-2 font-medium">{m['admin.nodeDetails.vmid']()}</th><th class="pb-2 font-medium">{m['admin.nodeDetails.name']()}</th><th class="pb-2 font-medium">{m['admin.nodeDetails.status']()}</th><th class="pb-2 font-medium">{m['admin.nodeDetails.vcpu']()}</th><th class="pb-2 font-medium">{m['admin.nodeDetails.memory']()}</th><th class="pb-2 text-right font-medium">{m['admin.nodeDetails.disk']()}</th></tr></thead>
							<tbody class="divide-y divide-border">{#each detail.containers.containers as container (container.vmid)}<tr><td class="py-2 font-mono">{container.vmid}</td><td class="py-2">{container.name || m['admin.nodeDetails.notReported']()}</td><td class="py-2">{guestStatusLabel(container.status)}</td><td class="py-2 font-mono tabular-nums">{container.cpuCount}</td><td class="py-2 font-mono tabular-nums">{formatBytes(container.memoryUsedBytes)} / {formatBytes(container.memoryTotalBytes)}</td><td class="py-2 text-right font-mono tabular-nums">{formatBytes(container.diskUsedBytes)} / {formatBytes(container.diskTotalBytes)}</td></tr>{/each}</tbody>
						</table></div>
					{/if}
				</section>
			</div>
		</Card>

		<Card pad="none" title={m['admin.nodeDetails.storageTitle']()} titleId="node-storage-section">
			{#if detail.inventory.storages.length === 0}
				<p class="px-5 py-4 text-sm text-muted-foreground">{m['admin.nodeDetails.storageEmpty']()}</p>
			{:else}
				<ul class="divide-y divide-border">
					{#each detail.inventory.storages as storage (`${storage.name}/${storage.type}`)}
						{@const percent = usagePercent(storage.usedBytes, storage.totalBytes)}
						<li class="space-y-2 px-5 py-3">
							<div class="flex flex-wrap items-center justify-between gap-2 text-sm"><span><span class="font-mono font-semibold">{storage.name}</span><span class="ml-2 text-xs text-muted-foreground">{storage.type}{#if storage.shared} · {m['admin.nodeDetails.shared']()}{/if}</span></span><span class="font-mono text-xs tabular-nums">{percent}%</span></div>
							{@render usageMeter(m['admin.nodeDetails.storageTitle'](), `${formatBytes(storage.usedBytes)} / ${formatBytes(storage.totalBytes)}`, percent)}
						</li>
					{/each}
				</ul>
			{/if}
		</Card>
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
