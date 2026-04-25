<script lang="ts">
	import { fade } from 'svelte/transition';
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
	import { CaretLeft } from 'phosphor-svelte';

	// Extracted components
	import VMActionBar from './_components/VMActionBar.svelte';
	import VMStatCards from './_components/VMStatCards.svelte';
	import ConsoleBanner from './_components/ConsoleBanner.svelte';
	import EditableDescription from './_components/EditableDescription.svelte';
	import TabOverview from './_tabs/TabOverview.svelte';
	import TabDisks from './_tabs/TabDisks.svelte';
	import TabNetwork from './_tabs/TabNetwork.svelte';
	import TabSnapshots from './_tabs/TabSnapshots.svelte';
	import TabCloudInit from './_tabs/TabCloudInit.svelte';

	const vmid = $derived(parseInt($page.params.id, 10));

	let loading = $state(true);
	let error = $state<Error | null>(null);
	let config = $state<VMConfig | null>(null);
	let metrics = $state<VMMetrics | null>(null);
	let snapshotData = $state<SnapshotList | null>(null);

	let activeTab = $state<'overview' | 'disks' | 'network' | 'snapshots' | 'cloudinit'>('overview');
	let actionLoading = $state(false);

	// Provisioning retry state
	const MAX_RETRIES = 6;
	const RETRY_DELAY_MS = 2000;
	let provisioning = $state(false);
	let retryCount = $state(0);
	let retryTimeout: ReturnType<typeof setTimeout> | null = null;
	let isLoading = $state(false);
	let lastVmid = $state(0);

	// Description editing
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
		if (isLoading) return;
		isLoading = true;
		loading = true;
		error = null;

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

	async function saveDescription(value: string) {
		if (!config) return;
		savingDescription = true;
		try {
			await updateVMConfig(vmid, { description: value });
			config = { ...config, description: value };
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
			if (isRunning) {
				deleteProgress = $t('vm.shuttingDown');
				await api.post(`/api/v1/vms/${vmid}/action`, { action: 'shutdown', node: config?.node });

				const MAX_POLL_ATTEMPTS = 30;
				const POLL_INTERVAL = 1000;
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

	function openConsole() {
		window.open(`/vm/${config?.vmid ?? vmid}/console?name=${encodeURIComponent(config?.name ?? String(vmid))}`, '_blank', 'width=1024,height=640,resizable=yes');
	}

	// Polling metrics with $effect
	$effect(() => {
		if (vmid === lastVmid) return;
		lastVmid = vmid;

		isLoading = false;
		if (retryTimeout) {
			clearTimeout(retryTimeout);
			retryTimeout = null;
		}
		retryCount = 0;
		provisioning = false;
		void load();

		const interval = setInterval(async () => {
			if (!config) return;
			try {
				metrics = await getVMMetrics(vmid);
			} catch {
				// best-effort
			}
		}, 5000);

		return () => {
			clearInterval(interval);
			if (retryTimeout) {
				clearTimeout(retryTimeout);
				retryTimeout = null;
			}
		};
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
				<span class="pv-badge {(metrics?.status ?? config.status) === 'running' ? 'pv-badge--online' : (metrics?.status ?? config.status) === 'stopped' ? 'pv-badge--offline' : 'pv-badge--warn'}">
					{$t(`common.statusMap.${metrics?.status ?? config.status}`, {
						default: metrics?.status ?? config.status
					})}
				</span>
			{/if}
		</div>
		{#if config}
			<VMActionBar
				{vmid}
				name={config.name}
				node={config.node}
				status={metrics?.status ?? config.status}
				{actionLoading}
				onAction={doAction}
				onRefresh={load}
				onConsole={openConsole}
				onDelete={() => (showDeleteDialog = true)}
			/>
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
		<VMStatCards {config} {metrics} />

		{#if (metrics?.status ?? config.status) === 'running'}
			<ConsoleBanner {vmid} name={config.name} onConsole={openConsole} />
		{/if}

		<EditableDescription
			value={config.description ?? ''}
			loading={savingDescription}
			onSave={saveDescription}
		/>

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
		{#key activeTab}
		<div in:fade={{ duration: 150 }}>
		{#if activeTab === 'overview'}
			<TabOverview {config} {metrics} onOpenHardware={openHardwareModal} />
		{/if}

		{#if activeTab === 'disks'}
			<TabDisks
				{config}
				{metrics}
				availableIsos={vmSettings?.availableIsos ?? []}
				onOpenAdd={openDiskAddModal}
				onOpenResize={openDiskResizeModal}
				onOpenDelete={openDiskDeleteModal}
				onRefresh={load}
			/>
		{/if}

		{#if activeTab === 'network'}
			<TabNetwork {config} />
		{/if}

		{#if activeTab === 'snapshots'}
			<TabSnapshots
				{snapshotData}
				{creatingSnapshot}
				showSnapshotForm={showSnapshotForm}
				snapName={snapName}
				snapDesc={snapDesc}
				onToggleForm={() => (showSnapshotForm = !showSnapshotForm)}
				onCreateSnapshot={doCreateSnapshot}
				onDeleteSnapshot={doDeleteSnapshot}
				onRollback={doRollback}
				onSnapNameChange={(v) => (snapName = v)}
				onSnapDescChange={(v) => (snapDesc = v)}
			/>
		{/if}

		{#if activeTab === 'cloudinit'}
			<TabCloudInit {config} />
		{/if}
		</div>
		{/key}
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
		currentMemMB={config.maxMemMb}
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
		maxDiskGB={vmSettings?.limits.maxDiskGb ?? 2000}
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
