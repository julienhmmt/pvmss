<script lang="ts">
	import { getVmDetailContext } from './detail.svelte';
	import VmActionBar from './VmActionBar.svelte';
	import DeleteVmDialog from './DeleteVmDialog.svelte';
	import Tabs from '$lib/shared/ui/Tabs.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';
	import VmDisksTab from './disks/VmDisksTab.svelte';
	import VmNetworkTab from './network/VmNetworkTab.svelte';
	import VmHardwareTab from './hardware/VmHardwareTab.svelte';
	import CloudInitTab from './CloudInitTab.svelte';
	import VmSnapshotsTab from './VmSnapshotsTab.svelte';
	import VmActivityTab from './VmActivityTab.svelte';
	import ConsoleBanner from './ConsoleBanner.svelte';
	import VmMetricsRow from './VmMetricsRow.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import { formatBytes } from '$lib/shared/format-bytes';

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
		{ id: 'snapshots', label: () => m['vms.detail.tabSnapshots']() },
		{ id: 'activity', label: () => m['vm.activity.tab']() }
	];
	let activeTab = $state('overview');

	const statusClasses: Record<string, string> = {
		running: 'bg-success-soft text-success-soft-foreground border-success-soft-border',
		stopped: 'bg-muted text-muted-foreground border-border',
		paused: 'bg-destructive-soft text-destructive-soft-foreground border-destructive-soft-border'
	};

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
</script>

{#if store.loading && store.entity === null}
	<div role="status" aria-live="polite" class="grid gap-6" data-testid="vm-detail-skeleton">
		<div class="grid gap-2">
			<Skeleton class="h-10 w-56" />
			<Skeleton class="h-4 w-64" />
		</div>
		<Skeleton class="h-10 w-48" />
		<div class="grid grid-cols-2 gap-4 sm:grid-cols-4">
			<Skeleton class="h-20 w-full" />
			<Skeleton class="h-20 w-full" />
			<Skeleton class="h-20 w-full" />
			<Skeleton class="h-20 w-full" />
		</div>
		<div class="grid gap-2">
			<Skeleton class="h-10 w-full" />
			<Skeleton class="h-32 w-full" />
		</div>
	</div>
{:else if store.error}
	<p role="alert" class="text-destructive" data-testid="vm-detail-error">{store.error}</p>
{:else if store.entity}
	<header class="rounded-xl border border-border bg-card p-6 shadow-card">
		<div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
			<div class="flex flex-wrap items-center gap-3">
				{#if editingName}
					<input
						type="text"
						class="rounded-md border border-border bg-background px-2 py-1 text-2xl font-semibold tracking-tight focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
						bind:value={nameDraft}
						onkeydown={handleNameKeydown}
						data-testid="vm-name-edit"
					/>
					<button
						type="button"
						class="rounded-lg bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
						onclick={() => void commitName()}
						data-testid="vm-name-save"
					>
						{m['common.save']()}
					</button>
					<button
						type="button"
						class="rounded-lg border border-border bg-background px-3 py-1.5 text-sm font-medium text-muted-foreground hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
						onclick={cancelName}
					>
						{m['common.cancel']()}
					</button>
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
					class="inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-medium {statusClasses[store.entity.status]}"
					aria-live="polite"
					data-testid="vm-status"
				>
					<span class="h-1.5 w-1.5 rounded-full bg-current" aria-hidden="true"></span>
					{store.entity.status}
				</span>
			</div>
			<div class="flex items-center gap-2">
				<ConsoleBanner />
			</div>
		</div>

		<p class="mt-3 font-mono text-sm text-muted-foreground" data-testid="vm-meta">
			{m['vms.detail.meta']({ vmid: String(store.entity.vmid), node: store.entity.node, pool: store.entity.pool })}
		</p>

		<div class="mt-5">
			<VmActionBar onDelete={() => (deleteOpen = true)} />
		</div>
	</header>

	<section class="mt-6 rounded-xl border border-border bg-card p-6 shadow-card">
		<div class="grid grid-cols-2 gap-6 md:grid-cols-4">
			<div data-testid="vm-stat-cpu">
				<p class="text-sm text-muted-foreground">{m['vms.detail.statCpu']()}</p>
				<p class="mt-1 text-2xl font-semibold font-mono">{store.entity.cpuCores} {m['common.cores']()}</p>
			</div>
			<div data-testid="vm-stat-memory">
				<p class="text-sm text-muted-foreground">{m['vms.detail.statMemory']()}</p>
				<p class="mt-1 text-2xl font-semibold font-mono">{formatBytes(store.entity.memoryTotal)}</p>
			</div>
			<div data-testid="vm-stat-disk">
				<p class="text-sm text-muted-foreground">{m['vms.detail.statDisk']()}</p>
				<p class="mt-1 text-2xl font-semibold font-mono">{formatBytes(store.entity.diskTotal)}</p>
			</div>
			<div data-testid="vm-stat-uptime">
				<p class="text-sm text-muted-foreground">{m['vms.detail.statUptime']()}</p>
				<p class="mt-1 text-2xl font-semibold font-mono">
					{#if store.entity.uptimeSeconds}
						{formatUptime(store.entity.uptimeSeconds)}
					{:else}
						{m['common.dash']()}
					{/if}
				</p>
			</div>
		</div>
	</section>

	<VmMetricsRow />

	<div class="mt-8">
		<Tabs {tabs} bind:active={activeTab} />

		<div
			id="panel-overview"
			role="tabpanel"
			aria-labelledby="tab-overview"
			hidden={activeTab !== 'overview'}
			class="mt-4 rounded-xl border border-border bg-card p-6 shadow-card"
		>
			<section>
				<h2 class="text-sm font-medium text-muted-foreground">{m['vms.detail.descriptionLabel']()}</h2>
				{#if editingDescription}
					<textarea
						class="pv-input mt-3"
						bind:value={descriptionDraft}
						onkeydown={(e) => {
						if (e.key === 'Escape') { e.preventDefault(); cancelDescription(); }
						if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') { e.preventDefault(); void commitDescription(); }
					}}
						rows="4"
						data-testid="vm-description-edit"
					></textarea>
					<div class="mt-3 flex gap-2">
						<button
							type="button"
							class="rounded-lg bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
							onclick={() => void commitDescription()}
							data-testid="vm-description-save"
						>
							{m['common.save']()}
						</button>
						<button
							type="button"
							class="rounded-lg border border-border bg-background px-3 py-1.5 text-sm font-medium text-muted-foreground hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
							onclick={cancelDescription}
						>
							{m['common.cancel']()}
						</button>
					</div>
				{:else}
					<button
						type="button"
						class="mt-3 w-full rounded-lg border border-dashed border-border bg-muted/30 px-4 py-4 text-left text-sm leading-6 hover:cursor-text hover:bg-muted/50"
						onclick={startEditDescription}
						title={m['vms.detail.clickToEdit']()}
						data-testid="vm-description"
					>
						{store.entity.description || m['vms.detail.noDescription']()}
					</button>
				{/if}
			</section>
		</div>

		<div id="panel-disks" role="tabpanel" aria-labelledby="tab-disks" hidden={activeTab !== 'disks'} class="mt-4">
			<VmDisksTab />
		</div>

		<div id="panel-network" role="tabpanel" aria-labelledby="tab-network" hidden={activeTab !== 'network'} class="mt-4">
			<VmNetworkTab />
		</div>

		<div id="panel-hardware" role="tabpanel" aria-labelledby="tab-hardware" hidden={activeTab !== 'hardware'} class="mt-4">
			<VmHardwareTab />
		</div>

		<div id="panel-cloudinit" role="tabpanel" aria-labelledby="tab-cloudinit" hidden={activeTab !== 'cloudinit'} class="mt-4">
			{#if activeTab === 'cloudinit'}
				<CloudInitTab />
			{/if}
		</div>

		<div id="panel-snapshots" role="tabpanel" aria-labelledby="tab-snapshots" hidden={activeTab !== 'snapshots'} class="mt-4">
			{#if activeTab === 'snapshots'}
				<VmSnapshotsTab />
			{/if}
		</div>

		<div id="panel-activity" role="tabpanel" aria-labelledby="tab-activity" hidden={activeTab !== 'activity'} class="mt-4">
			{#if activeTab === 'activity'}
				<VmActivityTab />
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
