<script lang="ts">
	import { getVmDetailContext } from './detail.svelte';
	import VmActionBar from './VmActionBar.svelte';
	import DeleteVmDialog from './DeleteVmDialog.svelte';
	import Tabs from '$lib/shared/ui/Tabs.svelte';
	import VmDisksTab from './disks/VmDisksTab.svelte';
	import VmNetworkTab from './network/VmNetworkTab.svelte';
	import VmHardwareTab from './hardware/VmHardwareTab.svelte';
	import CloudInitTab from './CloudInitTab.svelte';
	import VmSnapshotsTab from './VmSnapshotsTab.svelte';
	import ConsoleBanner from './ConsoleBanner.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getVmDetailContext();

	let deleteOpen = $state(false);
	let editingName = $state(false);
	let editingDescription = $state(false);
	let nameDraft = $state('');
	let descriptionDraft = $state('');

	const tabs = [
		{ id: 'overview', label: () => m['vms.detail.tabOverview']() },
		{ id: 'disks', label: () => m['vms.detail.tabDisks']() },
		{ id: 'network', label: () => m['vms.detail.tabNetwork']() },
		{ id: 'hardware', label: () => m['vms.detail.tabHardware']() },
		{ id: 'cloudinit', label: () => m['vms.detail.tabCloudinit']() },
		{ id: 'snapshots', label: () => m['vms.detail.tabSnapshots']() }
	];
	let activeTab = $state('overview');

	const statusClasses: Record<string, string> = {
		running: 'bg-success-soft text-success-soft-foreground',
		stopped: 'bg-muted text-muted-foreground',
		paused: 'bg-destructive-soft text-destructive-soft-foreground'
	};

	function formatBytes(bytes: number): string {
		const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
		let value = bytes;
		let unitIndex = 0;
		while (value >= 1024 && unitIndex < units.length - 1) {
			value /= 1024;
			unitIndex += 1;
		}
		return `${value.toFixed(1)} ${units[unitIndex]}`;
	}

	function formatUptime(seconds: number): string {
		const days = Math.floor(seconds / 86400);
		const hours = Math.floor((seconds % 86400) / 3600);
		const minutes = Math.floor((seconds % 3600) / 60);
		if (days > 0) return `${days}d ${hours}h`;
		if (hours > 0) return `${hours}h ${minutes}m`;
		return `${minutes}m`;
	}

	function startEditName(): void {
		if (store.entity === null) return;
		nameDraft = store.entity.name;
		editingName = true;
	}

	async function commitName(): Promise<void> {
		if (!editingName) return;
		editingName = false;
		if (store.entity === null || nameDraft === store.entity.name) return;
		await store.patch(nameDraft, null);
	}

	function cancelName(): void {
		editingName = false;
		nameDraft = '';
	}

	function startEditDescription(): void {
		if (store.entity === null) return;
		descriptionDraft = store.entity.description ?? '';
		editingDescription = true;
	}

	async function commitDescription(): Promise<void> {
		if (!editingDescription) return;
		editingDescription = false;
		if (store.entity === null) return;
		await store.patch(null, descriptionDraft);
	}

	function cancelDescription(): void {
		editingDescription = false;
		descriptionDraft = '';
	}

	function handleNameKeydown(event: KeyboardEvent): void {
		if (event.key === 'Enter') {
			event.preventDefault();
			void commitName();
		} else if (event.key === 'Escape') {
			event.preventDefault();
			cancelName();
		}
	}

	function handleDescriptionKeydown(event: KeyboardEvent): void {
		if (event.key === 'Enter') {
			event.preventDefault();
			void commitDescription();
		} else if (event.key === 'Escape') {
			event.preventDefault();
			cancelDescription();
		}
	}
</script>

{#if store.loading && store.entity === null}
	<p role="status" aria-live="polite" class="text-muted-foreground">{m['common.loading']()}</p>
{:else if store.error}
	<p role="alert" class="text-destructive" data-testid="vm-detail-error">{store.error}</p>
{:else if store.entity}
	<header class="mb-6">
		<div class="flex flex-wrap items-center gap-3">
			{#if editingName}
				<input
					type="text"
					class="rounded-md border border-border bg-background px-2 py-1 text-2xl font-semibold tracking-tight"
					bind:value={nameDraft}
					onkeydown={handleNameKeydown}
					onblur={commitName}
					data-testid="vm-name-edit"
				/>
			{:else}
				<button
					type="button"
					class="text-2xl font-semibold tracking-tight hover:cursor-text hover:underline"
					onclick={startEditName}
					title={m['vms.detail.clickToRename']()}
					data-testid="vm-name"
				>
					{store.entity.name}
				</button>
			{/if}
			<span
				class="inline-flex items-center rounded-full px-2 py-0.5 text-xs {statusClasses[store.entity.status]}"
				aria-live="polite"
				data-testid="vm-status"
			>
				{store.entity.status}
			</span>
		</div>
		<p class="mt-1 font-mono text-sm text-muted-foreground" data-testid="vm-meta">
			{m['vms.detail.meta']({ vmid: String(store.entity.vmid), node: store.entity.node, pool: store.entity.pool })}
		</p>
	</header>

	<VmActionBar onDelete={() => (deleteOpen = true)} />

	<div class="mt-4">
		<ConsoleBanner />
	</div>

	<dl class="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-4">
		<div class="rounded-md border border-border p-4" data-testid="vm-stat-cpu">
			<dt class="text-xs text-muted-foreground">{m['vms.detail.statCpu']()}</dt>
			<dd class="text-lg font-medium font-mono">{store.entity.cpuCores} {m['common.cores']()}</dd>
		</div>
		<div class="rounded-md border border-border p-4" data-testid="vm-stat-memory">
			<dt class="text-xs text-muted-foreground">{m['vms.detail.statMemory']()}</dt>
			<dd class="text-lg font-medium font-mono">{formatBytes(store.entity.memoryTotal)}</dd>
		</div>
		<div class="rounded-md border border-border p-4" data-testid="vm-stat-disk">
			<dt class="text-xs text-muted-foreground">{m['vms.detail.statDisk']()}</dt>
			<dd class="text-lg font-medium font-mono">{formatBytes(store.entity.diskTotal)}</dd>
		</div>
		<div class="rounded-md border border-border p-4" data-testid="vm-stat-uptime">
			<dt class="text-xs text-muted-foreground">{m['vms.detail.statUptime']()}</dt>
			<dd class="text-lg font-medium">
				{#if store.entity.uptimeSeconds}
					{formatUptime(store.entity.uptimeSeconds)}
				{:else}
					{m['common.dash']()}
				{/if}
			</dd>
		</div>
	</dl>

	<div class="mt-8">
		<Tabs {tabs} bind:active={activeTab} />

		<div id="panel-overview" role="tabpanel" aria-labelledby="tab-overview" hidden={activeTab !== 'overview'}>
			<section class="mt-6">
				<h2 class="mb-2 text-sm font-medium text-muted-foreground">{m['vms.detail.descriptionLabel']()}</h2>
				{#if editingDescription}
					<textarea
						class="w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
						bind:value={descriptionDraft}
						onkeydown={handleDescriptionKeydown}
						onblur={commitDescription}
						rows="3"
						data-testid="vm-description-edit"
					></textarea>
				{:else}
					<button
						type="button"
						class="w-full rounded-md border border-border bg-muted/30 px-3 py-2 text-left text-sm hover:cursor-text hover:bg-muted/50"
						onclick={startEditDescription}
						title={m['vms.detail.clickToEdit']()}
						data-testid="vm-description"
					>
						{store.entity.description || m['vms.detail.noDescription']()}
					</button>
				{/if}
			</section>
		</div>

		<div id="panel-disks" role="tabpanel" aria-labelledby="tab-disks" hidden={activeTab !== 'disks'} class="mt-6">
			<VmDisksTab />
		</div>

		<div id="panel-network" role="tabpanel" aria-labelledby="tab-network" hidden={activeTab !== 'network'} class="mt-6">
			<VmNetworkTab />
		</div>

		<div id="panel-hardware" role="tabpanel" aria-labelledby="tab-hardware" hidden={activeTab !== 'hardware'} class="mt-6">
			<VmHardwareTab />
		</div>

		<div id="panel-cloudinit" role="tabpanel" aria-labelledby="tab-cloudinit" hidden={activeTab !== 'cloudinit'} class="mt-6">
			{#if activeTab === 'cloudinit'}
				<CloudInitTab />
			{/if}
		</div>

		<div id="panel-snapshots" role="tabpanel" aria-labelledby="tab-snapshots" hidden={activeTab !== 'snapshots'} class="mt-6">
			{#if activeTab === 'snapshots'}
				<VmSnapshotsTab />
			{/if}
		</div>
	</div>

	{#if store.patchError}
		<p role="alert" class="mt-4 text-sm text-destructive" data-testid="vm-patch-error">
			{store.patchError}
		</p>
	{/if}

	<DeleteVmDialog bind:open={deleteOpen} />
{/if}
