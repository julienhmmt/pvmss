<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api/client';
	import { ApiRequestError } from '$lib/types/api';
	import {
		getVMConfig,
		getVMMetrics,
		getVMSnapshots,
		getVMSettings,
		createSnapshot,
		deleteSnapshot,
		rollbackSnapshot,
		updateVMConfig,
		deleteVM,
		type VMConfig,
		type VMMetrics,
		type VMSettings,
		type Snapshot,
		type SnapshotList
	} from '$lib/api/vm-details';
	import VMHardwareModal from '$lib/components/vm/VMHardwareModal.svelte';
	import VMDiskAddModal from '$lib/components/vm/VMDiskAddModal.svelte';
	import VMDiskResizeModal from '$lib/components/vm/VMDiskResizeModal.svelte';
	import VMDiskDeleteModal from '$lib/components/vm/VMDiskDeleteModal.svelte';
	import type { DiskInfo } from '$lib/api/vm-details';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import {
		Play,
		Stop,
		ArrowCounterClockwise,
		ArrowsClockwise,
		CaretLeft,
		Camera,
		Trash,
		ClockCounterClockwise,
		HardDrive,
		Cpu,
		Desktop,
		Network,
		Lock,
		CloudArrowUp,
		Monitor,
		ArrowSquareOut
	} from 'phosphor-svelte';

	const vmid = $derived(parseInt($page.params.id, 10));

	let loading = $state(true);
	let error = $state<Error | null>(null);
	let config = $state<VMConfig | null>(null);
	let metrics = $state<VMMetrics | null>(null);
	let snapshotData = $state<SnapshotList | null>(null);

	let activeTab = $state<'overview' | 'disks' | 'network' | 'snapshots' | 'cloudinit'>('overview');
	let actionLoading = $state(false);
	let metricsInterval: ReturnType<typeof setInterval> | null = null;

	// Provisioning retry state: when the VM was just created it may not yet be
	// visible in the backend cache. We retry silently up to MAX_RETRIES times.
	const MAX_RETRIES = 6;
	const RETRY_DELAY_MS = 2000;
	let provisioning = $state(false);
	let retryCount = $state(0);
	let retryTimeout: ReturnType<typeof setTimeout> | null = null;
	let isLoading = $state(false);

	// Edit states
	let editingDescription = $state(false);
	let descriptionDraft = $state('');
	let savingDescription = $state(false);

	// Hardware update
	let showHardwareModal = $state(false);
	let vmSettings = $state<VMSettings | null>(null);
	let hardwareReloadTimeout: ReturnType<typeof setTimeout> | null = null;

	// Disk management
	let showDiskAddModal = $state(false);
	let showDiskResizeModal = $state(false);
	let showDiskDeleteModal = $state(false);
	let selectedDisk = $state<DiskInfo | null>(null);

	$effect(() => {
		return () => {
			if (hardwareReloadTimeout !== null) {
				clearTimeout(hardwareReloadTimeout);
				hardwareReloadTimeout = null;
			}
		};
	});

	// Delete VM
	let showDeleteDialog = $state(false);
	let deleting = $state(false);
	let deleteProgress = $state('');

	// Snapshot creation
	let showSnapshotForm = $state(false);
	let snapName = $state('');
	let snapDesc = $state('');
	let creatingSnapshot = $state(false);

	async function load() {
		if (isLoading) return; // Prevent concurrent calls
		isLoading = true;
		loading = true;
		error = null;

		// Clear any existing retry timeout
		if (retryTimeout) {
			clearTimeout(retryTimeout);
			retryTimeout = null;
		}

		try {
			[config, snapshotData] = await Promise.all([getVMConfig(vmid), getVMSnapshots(vmid)]);
			metrics = await getVMMetrics(vmid);
			provisioning = false;
			retryCount = 0;
		} catch (e) {
			const isNotFound = e instanceof ApiRequestError && e.status === 404;
			if (isNotFound && retryCount < MAX_RETRIES) {
				provisioning = true;
				retryCount += 1;
				retryTimeout = setTimeout(() => {
					isLoading = false;
					load();
				}, RETRY_DELAY_MS);
			} else {
				provisioning = false;
				error = e as Error;
			}
		} finally {
			isLoading = false;
			loading = false;
		}
	}

	async function refreshMetrics() {
		if (!config) return;
		try {
			metrics = await getVMMetrics(vmid);
		} catch {
			// best-effort
		}
	}

	async function doAction(action: string) {
		if (!config) return;
		actionLoading = true;
		try {
			await api.post(`/api/v1/vms/${vmid}/action`, { action, node: config.node });
			toast.success(`${action} sent to ${config.name || vmid}`);
			setTimeout(() => load(), 2000);
		} catch {
			toast.error(`Failed to ${action} VM ${vmid}`);
		} finally {
			actionLoading = false;
		}
	}

	async function openHardwareModal() {
		try {
			vmSettings = await getVMSettings(vmid);
		} catch {
			toast.error($t('common.error'));
			return;
		}
		showHardwareModal = true;
	}

	async function openDiskAddModal() {
		if (!vmSettings) {
			try {
				vmSettings = await getVMSettings(vmid);
			} catch {
				toast.error($t('common.error'));
				return;
			}
		}
		showDiskAddModal = true;
	}

	function openDiskResizeModal(disk: DiskInfo) {
		selectedDisk = disk;
		showDiskResizeModal = true;
	}

	function openDiskDeleteModal(disk: DiskInfo) {
		selectedDisk = disk;
		showDiskDeleteModal = true;
	}

	async function saveDescription() {
		if (!config) return;
		savingDescription = true;
		try {
			await updateVMConfig(vmid, { description: descriptionDraft });
			config = { ...config, description: descriptionDraft };
			editingDescription = false;
			toast.success($t('vm.descriptionSaved'));
		} catch {
			toast.error($t('common.error'));
		} finally {
			savingDescription = false;
		}
	}

	async function doCreateSnapshot() {
		if (!snapName.trim()) return;
		creatingSnapshot = true;
		try {
			await createSnapshot(vmid, snapName.trim(), snapDesc.trim());
			toast.success($t('vm.snapshotCreated', { values: { name: snapName } }));
			snapName = '';
			snapDesc = '';
			showSnapshotForm = false;
			snapshotData = await getVMSnapshots(vmid);
		} catch {
			toast.error($t('common.error'));
		} finally {
			creatingSnapshot = false;
		}
	}

	async function doDeleteSnapshot(name: string) {
		try {
			await deleteSnapshot(vmid, name);
			toast.success($t('vm.snapshotDeleted', { values: { name } }));
			snapshotData = await getVMSnapshots(vmid);
		} catch {
			toast.error($t('common.error'));
		}
	}

	async function confirmDeleteVM() {
		const vmStatus = metrics?.status ?? config?.status;
		const isRunning = vmStatus === 'running';

		deleting = true;
		deleteProgress = '';

		try {
			// If VM is running, shutdown first
			if (isRunning) {
				deleteProgress = $t('vm.shuttingDown');
				await api.post(`/api/v1/vms/${vmid}/action`, { action: 'shutdown', node: config?.node });

				// Poll for stopped state
				const MAX_POLL_ATTEMPTS = 30; // 30 seconds max
				const POLL_INTERVAL = 1000; // 1 second
				let attempts = 0;

				while (attempts < MAX_POLL_ATTEMPTS) {
					await new Promise((resolve) => setTimeout(resolve, POLL_INTERVAL));
					attempts++;

					try {
						const updatedConfig = await getVMConfig(vmid);
						const currentStatus = updatedConfig.status;
						if (currentStatus === 'stopped') {
							deleteProgress = $t('vm.stopped');
							break;
						}
						deleteProgress = $t('vm.shuttingDownProgress', { values: { attempts, max: MAX_POLL_ATTEMPTS } });
					} catch {
						// Continue polling on error
					}
				}


				// Verify it's stopped before proceeding
				const finalConfig = await getVMConfig(vmid);
				if (finalConfig.status !== 'stopped') {
					throw new Error($t('vm.deleteFailed'));
				}
			}

			deleteProgress = $t('vm.deleting');
			await deleteVM(vmid);
			toast.success($t('vm.deleteSuccess', { values: { name: config?.name ?? String(vmid) } }));
			goto('/home');
		} catch {
			toast.error($t('vm.deleteFailed'));
			deleting = false;
			deleteProgress = '';
			showDeleteDialog = false;
		}
	}

	async function doRollback(name: string) {
		try {
			await rollbackSnapshot(vmid, name);
			toast.success($t('vm.snapshotRolledBack', { values: { name } }));
			await load();
		} catch {
			toast.error($t('common.error'));
		}
	}

	function statusClass(status: string) {
		if (status === 'running') return 'pv-badge--online';
		if (status === 'stopped') return 'pv-badge--offline';
		return 'pv-badge--warn';
	}

	function uptimeLabel(seconds: number): string {
		if (!seconds) return '—';
		const d = Math.floor(seconds / 86400);
		const h = Math.floor((seconds % 86400) / 3600);
		if (d > 0) return `${d}d ${h}h`;
		const m = Math.floor((seconds % 3600) / 60);
		return h > 0 ? `${h}h ${m}m` : `${m}m`;
	}

	function snapshotDate(ts: number): string {
		if (!ts) return '—';
		return new Date(ts * 1000).toLocaleString();
	}

	onMount(() => {
		load();
		metricsInterval = setInterval(refreshMetrics, 5000);
	});

	onDestroy(() => {
		if (metricsInterval) clearInterval(metricsInterval);
		if (retryTimeout) clearTimeout(retryTimeout);
	});
</script>

<svelte:head>
	<title>PVMSS — {config?.name ?? `VM ${vmid}`}</title>
</svelte:head>

<div class="mx-auto px-4 py-6 pv-content-width">
	<!-- Back + Header -->
	<div class="mb-5 flex items-center justify-between">
		<div class="flex items-center gap-3">
			<button
				class="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
				onclick={() => goto('/home')}
			>
				<CaretLeft class="h-4 w-4" />
				{$t('nav.myVms')}
			</button>
			{#if config}
				<span class="text-muted-foreground">/</span>
				<span class="font-semibold">{config.name || `VM ${config.vmid}`}</span>
				<span class="pv-badge {statusClass(metrics?.status ?? config.status)}">
					{$t(`common.statusMap.${metrics?.status ?? config.status}`, {
						default: metrics?.status ?? config.status
					})}
				</span>
			{/if}
		</div>
		{#if config}
			<div class="flex items-center gap-2">
				<button
					class="pv-action-btn"
					onclick={() => load()}
					disabled={actionLoading}
					title={$t('common.refresh')}
				>
					<ArrowsClockwise class="h-4 w-4" />
				</button>
				{#if (metrics?.status ?? config.status) === 'stopped'}
					<button
						class="pv-action-btn pv-action-btn--start"
						onclick={() => doAction('start')}
						disabled={actionLoading}
						title={$t('vms.actions.start')}
					>
						<Play class="h-4 w-4" weight="fill" />
					</button>
				{:else if (metrics?.status ?? config.status) === 'running'}
					<button
						class="pv-action-btn pv-action-btn--stop"
						onclick={() => doAction('shutdown')}
						disabled={actionLoading}
						title={$t('vms.actions.shutdown')}
					>
						<Stop class="h-4 w-4" weight="fill" />
					</button>
					<button
						class="pv-action-btn pv-action-btn--halt"
						onclick={() => doAction('stop')}
						disabled={actionLoading}
						title={$t('vms.actions.forceStop')}
					>
						<Stop class="h-4 w-4" />
					</button>
					<button
						class="pv-action-btn"
						onclick={() => doAction('reboot')}
						disabled={actionLoading}
						title={$t('vms.actions.reboot')}
					>
						<ArrowCounterClockwise class="h-4 w-4" />
					</button>
				{/if}
			{#if (metrics?.status ?? config.status) === 'running'}
				<button
					class="pv-action-btn"
					onclick={() => window.open(`/vm/${config?.vmid ?? vmid}/console?name=${encodeURIComponent(config?.name ?? String(vmid))}`, '_blank', 'width=1024,height=640,resizable=yes')}
					title={$t('vm.openConsole')}
				>
					<Monitor class="h-4 w-4" />
				</button>
			{/if}
			<button
				class="pv-action-btn pv-action-btn--stop"
				onclick={() => (showDeleteDialog = true)}
				disabled={actionLoading}
				title={$t('vm.delete')}
			>
				<Trash class="h-4 w-4" />
			</button>
			</div>
		{/if}
	</div>

	{#if provisioning || (loading && retryCount > 0)}
		<div class="flex flex-col items-center gap-4 py-16 text-center">
			<div class="pv-provisioning-spinner"></div>
			<p class="text-sm font-medium">{$t('vm.provisioning')}</p>
			<p class="text-xs text-muted-foreground">{$t('vm.provisioningHint')}</p>
		</div>
	{:else if error}
		<ErrorBanner {error} onRetry={() => { retryCount = 0; load(); }} />
	{:else if loading}
		<LoadingSkeleton variant="card" rows={6} />
	{:else if config}
		<!-- Stat cards -->
		<div class="mb-6 grid grid-cols-2 gap-3 sm:grid-cols-4">
			<div class="pv-stat-card">
				<Cpu class="pv-stat-icon" />
				<div>
					<div class="pv-stat-label">{$t('vms.cpu')}</div>
					<div class="pv-stat-value">
						{#if (metrics?.status ?? config.status) === 'running' && metrics}
							{Math.round(metrics.cpu * 100)}%
						{:else}
							{config.cpus} {$t('vm.cores')}
						{/if}
					</div>
				</div>
			</div>
			<div class="pv-stat-card">
				<Desktop class="pv-stat-icon" />
				<div>
					<div class="pv-stat-label">{$t('vms.ram')}</div>
					<div class="pv-stat-value">
						{#if (metrics?.status ?? config.status) === 'running' && metrics}
							{Math.round(metrics.mem_mb / 1024)} / {Math.round(metrics.max_mem_mb / 1024)} GB
						{:else}
							{Math.round(config.max_mem_mb / 1024)} GB
						{/if}
					</div>
				</div>
			</div>
			<div class="pv-stat-card">
				<HardDrive class="pv-stat-icon" />
				<div>
					<div class="pv-stat-label">{$t('common.storage')}</div>
					<div class="pv-stat-value">
						{config.disks.reduce((s, d) => s + d.size_gb, 0)} GB
					</div>
				</div>
			</div>
			<div class="pv-stat-card">
				<Network class="pv-stat-icon" />
				<div>
					<div class="pv-stat-label">{$t('admin.vmbr.iface')}</div>
					<div class="pv-stat-value">
						{config.networks.length}
						{$t('vm.interfaces')}
					</div>
				</div>
			</div>
		</div>

		<!-- Console banner (running VMs only) -->
		{#if (metrics?.status ?? config.status) === 'running'}
			<button
				class="pv-console-banner mb-4"
				onclick={() => window.open(`/vm/${config?.vmid ?? vmid}/console?name=${encodeURIComponent(config?.name ?? String(vmid))}`, '_blank', 'width=1024,height=640,resizable=yes')}
			>
				<Monitor class="h-5 w-5" />
				<span>{$t('vm.openConsole')}</span>
				<ArrowSquareOut class="pv-console-banner-arrow h-4 w-4" />
			</button>
		{/if}

		<!-- Description -->
		<div class="pv-table-wrap mb-4 p-4">
			<div class="mb-1 flex items-center justify-between">
				<span class="text-sm font-medium">{$t('common.description')}</span>
				{#if !editingDescription}
					<button
						class="text-xs text-muted-foreground hover:text-foreground"
						onclick={() => {
							descriptionDraft = config?.description ?? '';
							editingDescription = true;
						}}>{$t('common.edit')}</button
					>
				{/if}
			</div>
			{#if editingDescription}
				<textarea
					class="w-full rounded border border-border bg-background p-2 text-sm"
					rows="3"
					bind:value={descriptionDraft}
				></textarea>
				<div class="mt-2 flex gap-2">
					<button
						class="pv-btn-primary text-xs"
						onclick={saveDescription}
						disabled={savingDescription}
					>
						{savingDescription ? $t('common.saving') : $t('common.save')}
					</button>
					<button
						class="text-xs text-muted-foreground hover:text-foreground"
						onclick={() => (editingDescription = false)}>{$t('common.cancel')}</button
					>
				</div>
			{:else}
				<p class="text-sm text-muted-foreground">
					{config.description || $t('vm.noDescription')}
				</p>
			{/if}
		</div>

		<!-- Tabs -->
		<div class="mb-4 flex gap-1 border-b border-border">
			{#each [
				{ key: 'overview', label: $t('vm.tabOverview') },
				{ key: 'disks', label: $t('vm.tabDisks') },
				{ key: 'network', label: $t('vm.tabNetwork') },
				{ key: 'snapshots', label: $t('vm.tabSnapshots') },
				{ key: 'cloudinit', label: $t('vm.tabCloudInit') }
			] as tab (tab.key)}
				<button
					class="pv-tab {activeTab === tab.key ? 'pv-tab--active' : ''}"
					onclick={() => (activeTab = tab.key as typeof activeTab)}
				>
					{tab.label}
				</button>
			{/each}
		</div>

		<!-- Tab content -->
		{#if activeTab === 'overview'}
			<div class="pv-table-wrap">
				<table class="pv-table">
					<tbody>
						<tr class="pv-row">
							<td class="pv-td-label">VMID</td>
							<td class="pv-td-mono">{config.vmid}</td>
						</tr>
						<tr class="pv-row">
							<td class="pv-td-label">{$t('common.name')}</td>
							<td>{config.name || '—'}</td>
						</tr>
						<tr class="pv-row">
							<td class="pv-td-label">{$t('common.node')}</td>
							<td class="pv-td-mono">{config.node}</td>
						</tr>
						<tr class="pv-row">
							<td class="pv-td-label">{$t('common.status')}</td>
							<td>
								<span class="pv-badge {statusClass(metrics?.status ?? config.status)}">
									{$t(`common.statusMap.${metrics?.status ?? config.status}`, {
										default: metrics?.status ?? config.status
									})}
								</span>
							</td>
						</tr>
						<tr class="pv-row">
							<td class="pv-td-label">{$t('vms.uptime')}</td>
							<td class="pv-td-mono">{uptimeLabel(config.uptime)}</td>
						</tr>
						<tr class="pv-row">
							<td class="pv-td-label">{$t('admin.vms.cpus')}</td>
							<td class="pv-td-mono">{config.cpus}</td>
						</tr>
						<tr class="pv-row">
							<td class="pv-td-label">{$t('vms.ram')}</td>
							<td class="pv-td-mono">{Math.round(config.max_mem_mb / 1024)} GB</td>
						</tr>
						<tr class="pv-row">
							<td class="pv-td-label">{$t('admin.vms.tags')}</td>
							<td>
								{#if config.tags}
									<div class="flex flex-wrap gap-1">
										{#each config.tags.split(';').filter(Boolean) as tag (tag)}
											<span class="pv-badge">{tag.trim()}</span>
										{/each}
									</div>
								{:else}
									<span class="text-muted-foreground">—</span>
								{/if}
							</td>
						</tr>
						{#if config.efi_enabled}
							<tr class="pv-row">
								<td class="pv-td-label">{$t('vm.efi')}</td>
								<td>
									<Lock class="inline h-4 w-4 text-green-600" />
									{$t('common.enabled')}
								</td>
							</tr>
						{/if}
						{#if config.tpm_enabled}
							<tr class="pv-row">
								<td class="pv-td-label">{$t('vm.tpm')}</td>
								<td>
									<Lock class="inline h-4 w-4 text-green-600" />
									{$t('common.enabled')}
								</td>
							</tr>
						{/if}
					</tbody>
				</table>
				<div class="border-t border-border px-4 py-3">
					<button
						class="inline-flex items-center gap-1 text-sm text-primary hover:underline"
						onclick={openHardwareModal}
					>
						{$t('vm.hardware.modifyHardware')}
					</button>
				</div>
			</div>
		{/if}

		{#if activeTab === 'disks'}
			{@const isRunning = (metrics?.status ?? config.status) === 'running'}
			<div class="pv-table-wrap">
				<div class="flex items-center justify-between border-b border-border px-4 py-3">
					<span class="text-sm font-medium">{$t('vm.tabDisks')}</span>
					<button
						class="inline-flex items-center gap-1 rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
						onclick={openDiskAddModal}
						disabled={isRunning}
						title={isRunning ? $t('vm.disk.vmRunningWarning') : ''}
					>
						+ {$t('vm.disk.add')}
					</button>
				</div>
				{#if config.disks.length === 0}
					<p class="py-8 text-center text-sm text-muted-foreground">{$t('vm.noDisks')}</p>
				{:else}
					<table class="pv-table">
						<thead>
							<tr>
								<th>{$t('common.name')}</th>
								<th>{$t('common.storage')}</th>
								<th>{$t('common.size')}</th>
								<th>{$t('common.actions')}</th>
							</tr>
						</thead>
						<tbody>
							{#each config.disks as disk (disk.index)}
								<tr class="pv-row">
									<td class="pv-td-mono">
										{disk.index}
										{#if disk.is_boot}
											<span class="ml-1 text-[10px] font-medium text-amber-600">({$t('vm.disk.boot')})</span>
										{/if}
									</td>
									<td>{disk.storage || '—'}</td>
									<td class="pv-td-mono">{disk.size_gb} GB</td>
									<td>
										<div class="flex items-center gap-2">
											<button
												class="text-xs text-primary hover:underline disabled:opacity-50 disabled:cursor-not-allowed"
												onclick={() => openDiskResizeModal(disk)}
											>
												{$t('vm.disk.resize')}
											</button>
											<button
												class="text-xs text-destructive hover:underline disabled:opacity-50 disabled:cursor-not-allowed"
												onclick={() => openDiskDeleteModal(disk)}
												disabled={isRunning || disk.is_boot}
												title={isRunning ? $t('vm.disk.vmRunningWarning') : disk.is_boot ? $t('vm.disk.bootDiskWarning') : ''}
											>
												{$t('vm.disk.detach')}
											</button>
										</div>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
				{#if config.has_cdrom}
					<div class="border-t border-border px-4 py-2 text-sm text-muted-foreground">
						CD-ROM: {config.current_iso || $t('vm.noISO')}
					</div>
				{/if}
			</div>
		{/if}

		{#if activeTab === 'network'}
			<div class="pv-table-wrap">
				{#if config.networks.length === 0}
					<p class="py-8 text-center text-sm text-muted-foreground">{$t('vm.noNetworks')}</p>
				{:else}
					<table class="pv-table">
						<thead>
							<tr>
								<th>{$t('vm.interface')}</th>
								<th>{$t('vm.model')}</th>
								<th>{$t('admin.vmbr.iface')}</th>
								<th>MAC</th>
								<th>IP</th>
							</tr>
						</thead>
						<tbody>
							{#each config.networks as net, i (i)}
								<tr class="pv-row">
									<td class="pv-td-mono">{net.index || `net${i}`}</td>
									<td>{net.model || '—'}</td>
									<td class="pv-td-mono">{net.bridge || '—'}</td>
									<td class="pv-td-mono text-xs">{net.mac || '—'}</td>
									<td class="pv-td-mono text-xs">
										{#if net.ips && net.ips.length > 0}
											{net.ips.join(', ')}
										{:else}
											<span class="text-muted-foreground">—</span>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
			</div>
		{/if}

		{#if activeTab === 'snapshots'}
			<div class="pv-table-wrap">
				<div class="flex items-center justify-between border-b border-border px-4 py-3">
					<span class="text-sm font-medium">
						{snapshotData
							? `${snapshotData.snapshots.filter((s) => !s.current).length} / ${snapshotData.max_allowed} ${$t('vm.snapshots')}`
							: $t('vm.snapshots')}
					</span>
					{#if !showSnapshotForm && snapshotData && snapshotData.snapshots.filter((s) => !s.current).length < snapshotData.max_allowed}
						<button
							class="inline-flex items-center gap-1 text-sm text-primary hover:underline"
							onclick={() => (showSnapshotForm = true)}
						>
							<Camera class="h-4 w-4" />
							{$t('vm.createSnapshot')}
						</button>
					{/if}
				</div>

				{#if showSnapshotForm}
					<div class="border-b border-border px-4 py-3">
						<div class="mb-2 flex gap-2">
							<input
								class="flex-1 rounded border border-border bg-background px-2 py-1 text-sm"
								placeholder={$t('vm.snapshotNamePlaceholder')}
								bind:value={snapName}
							/>
							<input
								class="flex-1 rounded border border-border bg-background px-2 py-1 text-sm"
								placeholder={$t('common.description')}
								bind:value={snapDesc}
							/>
						</div>
						<div class="flex gap-2">
							<button
								class="pv-btn-primary text-xs"
								onclick={doCreateSnapshot}
								disabled={creatingSnapshot || !snapName.trim()}
							>
								{creatingSnapshot ? $t('common.saving') : $t('common.create')}
							</button>
							<button
								class="text-xs text-muted-foreground hover:text-foreground"
								onclick={() => (showSnapshotForm = false)}>{$t('common.cancel')}</button
							>
						</div>
					</div>
				{/if}

				{#if !snapshotData || snapshotData.snapshots.filter((s) => !s.current).length === 0}
					<p class="py-8 text-center text-sm text-muted-foreground">{$t('vm.noSnapshots')}</p>
				{:else}
					<table class="pv-table">
						<thead>
							<tr>
								<th>{$t('common.name')}</th>
								<th>{$t('common.description')}</th>
								<th>{$t('vm.snapshotDate')}</th>
								<th></th>
							</tr>
						</thead>
						<tbody>
							{#each snapshotData.snapshots.filter((s) => !s.current) as snap (snap.name)}
								<tr class="pv-row">
									<td class="pv-td-mono">{snap.name}</td>
									<td class="text-sm text-muted-foreground">{snap.description || '—'}</td>
									<td class="pv-td-mono text-xs">{snapshotDate(snap.snaptime)}</td>
									<td>
										<div class="flex items-center gap-1">
											<button
												class="pv-action-btn"
												onclick={() => doRollback(snap.name)}
												title={$t('vm.rollback')}
											>
												<ClockCounterClockwise class="h-3.5 w-3.5" />
											</button>
											<button
												class="pv-action-btn pv-action-btn--stop"
												onclick={() => doDeleteSnapshot(snap.name)}
												title={$t('common.delete')}
											>
												<Trash class="h-3.5 w-3.5" />
											</button>
										</div>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
			</div>
		{/if}

		{#if activeTab === 'cloudinit'}
			<div class="pv-table-wrap">
				{#if !config.cloud_init}
					<div class="flex flex-col items-center py-12 text-muted-foreground">
						<CloudArrowUp class="mb-3 h-10 w-10 opacity-30" />
						<p class="text-sm">{$t('vm.noCloudInit')}</p>
					</div>
				{:else}
					<table class="pv-table">
						<tbody>
							{#if config.cloud_init.user}
								<tr class="pv-row">
									<td class="pv-td-label">{$t('admin.cloudinit.username')}</td>
									<td class="pv-td-mono">{config.cloud_init.user}</td>
								</tr>
							{/if}
							{#if config.cloud_init.ip_config}
								<tr class="pv-row">
									<td class="pv-td-label">{$t('vm.ipConfig')}</td>
									<td class="pv-td-mono text-xs">{config.cloud_init.ip_config}</td>
								</tr>
							{/if}
							{#if config.cloud_init.nameserver}
								<tr class="pv-row">
									<td class="pv-td-label">{$t('vm.nameserver')}</td>
									<td class="pv-td-mono">{config.cloud_init.nameserver}</td>
								</tr>
							{/if}
							{#if config.cloud_init.ssh_keys}
								<tr class="pv-row">
									<td class="pv-td-label">{$t('vm.sshKeys')}</td>
									<td>
										<pre class="max-h-40 overflow-auto rounded bg-muted p-2 text-xs">{config
												.cloud_init.ssh_keys}</pre>
									</td>
								</tr>
							{/if}
						</tbody>
					</table>
				{/if}
			</div>
		{/if}
	{/if}
</div>

<!-- Hardware update modal -->
{#if showHardwareModal && config}
	<VMHardwareModal
		bind:open={showHardwareModal}
		vmid={config.vmid}
		node={config.node}
		currentSockets={config.sockets}
		currentCores={config.cores}
		currentMemMB={config.max_mem_mb}
		currentTags={config.tags}
		currentNetworks={config.networks}
		isRunning={(metrics?.status ?? config.status) === 'running'}
		settings={vmSettings}
		onclose={() => (showHardwareModal = false)}
		onsuccess={() => {
			showHardwareModal = false;
			hardwareReloadTimeout = setTimeout(() => {
				hardwareReloadTimeout = null;
				load();
			}, 1500);
		}}
	/>
{/if}

<!-- Disk management modals -->
{#if showDiskAddModal && config}
	<VMDiskAddModal
		bind:open={showDiskAddModal}
		vmid={config.vmid}
		settings={vmSettings}
		currentDiskCount={config.disks.length}
		onclose={() => (showDiskAddModal = false)}
		onsuccess={() => {
			showDiskAddModal = false;
			load();
		}}
	/>
{/if}

{#if showDiskResizeModal && config && selectedDisk}
	<VMDiskResizeModal
		bind:open={showDiskResizeModal}
		vmid={config.vmid}
		disk={selectedDisk}
		maxDiskGB={vmSettings?.limits.max_disk_gb ?? 2000}
		onclose={() => (showDiskResizeModal = false)}
		onsuccess={() => {
			showDiskResizeModal = false;
			selectedDisk = null;
			load();
		}}
	/>
{/if}

{#if showDiskDeleteModal && config && selectedDisk}
	<VMDiskDeleteModal
		bind:open={showDiskDeleteModal}
		vmid={config.vmid}
		disk={selectedDisk}
		vmStatus={metrics?.status ?? config?.status ?? 'stopped'}
		onclose={() => (showDiskDeleteModal = false)}
		onsuccess={() => {
			showDiskDeleteModal = false;
			selectedDisk = null;
			load();
		}}
	/>
{/if}

<!-- Delete VM confirmation dialog -->
<AlertDialog.Root bind:open={showDeleteDialog}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{$t('vm.deleteConfirmTitle')}</AlertDialog.Title>
			<AlertDialog.Description>
				{#if !deleting}
					{$t('vm.deleteConfirmMessage', { values: { name: config?.name ?? String(vmid), vmid } })}
					{#if (metrics?.status ?? config?.status) === 'running'}
						<p class="mt-2 font-medium text-destructive">{$t('vm.deleteConfirmRunning')}</p>
					{/if}
				{:else}
					<p class="text-sm font-medium">{deleteProgress || $t('common.loading')}</p>
				{/if}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={deleting}>{$t('common.cancel')}</AlertDialog.Cancel>
			<AlertDialog.Action
				class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
				disabled={deleting}
				onclick={confirmDeleteVM}
			>
				{deleting ? $t('common.loading') : $t('common.delete')}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<style>
	:global(.pv-stat-card) {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.875rem 1rem;
		background: var(--card);
		border: 1px solid var(--border);
		border-radius: 0.5rem;
	}
	:global(.pv-stat-icon) {
		width: 1.25rem;
		height: 1.25rem;
		flex-shrink: 0;
		color: var(--muted-foreground);
	}
	:global(.pv-stat-label) {
		font-size: 0.75rem;
		color: var(--muted-foreground);
		line-height: 1.2;
	}
	:global(.pv-stat-value) {
		font-size: 0.9rem;
		font-weight: 600;
		line-height: 1.4;
	}
	:global(.pv-tab) {
		padding: 0.5rem 0.875rem;
		font-size: 0.875rem;
		color: var(--muted-foreground);
		border-bottom: 2px solid transparent;
		cursor: pointer;
		background: none;
		border-top: none;
		border-left: none;
		border-right: none;
		transition: color 0.12s, border-color 0.12s;
	}
	:global(.pv-tab:hover) {
		color: var(--foreground);
	}
	:global(.pv-tab--active) {
		color: var(--foreground);
		border-bottom-color: var(--primary);
		font-weight: 500;
	}
	:global(.pv-td-label) {
		width: 10rem;
		font-size: 0.8rem;
		color: var(--muted-foreground);
		white-space: nowrap;
	}
	:global(.pv-console-banner) {
		display: flex;
		align-items: center;
		gap: 0.625rem;
		width: 100%;
		padding: 0.875rem 1.25rem;
		background: hsl(217 91% 60% / 0.08);
		border: 1px solid hsl(217 91% 60% / 0.25);
		border-radius: 0.5rem;
		color: hsl(217 91% 55%);
		font-size: 0.9375rem;
		font-weight: 500;
		cursor: pointer;
		transition: background 0.15s, border-color 0.15s;
		text-align: left;
	}
	:global(.pv-console-banner:hover) {
		background: hsl(217 91% 60% / 0.15);
		border-color: hsl(217 91% 60% / 0.5);
	}
	:global(.pv-console-banner-arrow) {
		margin-left: auto;
		opacity: 0.6;
	}
	:global(.pv-provisioning-spinner) {
		width: 2.5rem;
		height: 2.5rem;
		border: 3px solid var(--border);
		border-top-color: var(--primary);
		border-radius: 50%;
		animation: pv-spin 0.8s linear infinite;
	}
	@keyframes pv-spin {
		to { transform: rotate(360deg); }
	}
	:global(.pv-btn-primary) {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		padding: 0.375rem 0.75rem;
		background: var(--primary);
		color: var(--primary-foreground);
		border-radius: 0.375rem;
		font-size: 0.8125rem;
		font-weight: 500;
		border: none;
		cursor: pointer;
		transition: opacity 0.12s;
	}
	:global(.pv-btn-primary:disabled) {
		opacity: 0.5;
		cursor: not-allowed;
	}
	:global(.pv-action-btn--halt:hover:not(:disabled)) {
		background: hsl(0 84% 50% / 0.2);
		border-color: hsl(0 84% 50% / 0.6);
		color: hsl(0 84% 40%);
	}
</style>
