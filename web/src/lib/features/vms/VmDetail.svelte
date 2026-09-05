<script lang="ts">
	import { getVmDetailContext } from './detail.svelte';
	import VmActionBar from './VmActionBar.svelte';
	import DeleteVmDialog from './DeleteVmDialog.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
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
	import Button from '$lib/shared/ui/Button.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import StatCard from '$lib/shared/ui/StatCard.svelte';
	import { focusOnMount } from '$lib/shared/ui/focus-on-mount';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import { formatBytes } from '$lib/shared/format-bytes';

	const store = getVmDetailContext();
	const session = getSessionContext();

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

	// Same mapping as the VM list. It used to differ here — paused rendered in
	// the destructive triple on this page and in the warning triple in the
	// list, so the same VM changed colour depending on where you looked at
	// it. Paused is a caution, not a failure. Anything unrecognised falls
	// back to the neutral tone rather than rendering an unstyled badge.
	function statusTone(status: string): 'ok' | 'off' | 'warn' {
		if (status === 'running') return 'ok';
		if (status === 'paused') return 'warn';
		return 'off';
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
	<Alert data-testid="vm-detail-error">{store.error}</Alert>
{:else if store.entity}
	<header class="rounded-xl border border-border bg-card p-6 shadow-card">
		<div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
			<div class="flex flex-wrap items-center gap-3">
				{#if editingName}
					<input
						type="text"
						class="pv-input w-auto max-w-full text-2xl font-semibold tracking-tight"
						bind:value={nameDraft}
						onkeydown={handleNameKeydown}
						data-testid="vm-name-edit"
					/>
					<Button size="sm" onclick={() => void commitName()} data-testid="vm-name-save">
						{m['common.save']()}
					</Button>
					<Button variant="secondary" size="sm" onclick={cancelName}>
						{m['common.cancel']()}
					</Button>
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
				<span aria-live="polite" data-testid="vm-status">
					<Pill tone={statusTone(store.entity.status)} size="md" label={store.entity.status} />
				</span>
				{#if store.entity.lock}
					<span data-testid="vm-lock-badge">
						<Pill tone="warn" size="md" dot={false} label={m['vms.lock.badge']({ lock: store.entity.lock })} />
					</span>
				{/if}
			</div>
			<div class="flex items-center gap-2">
				<ConsoleBanner />
			</div>
		</div>

		<p class="mt-3 font-mono text-sm text-muted-foreground" data-testid="vm-meta">
			{m['vms.detail.meta']({ vmid: String(store.entity.vmid), node: store.entity.node, pool: store.entity.pool })}
		</p>

		{#if store.entity.lock && session.isAdmin}
			<p class="mt-2 font-mono text-xs text-muted-foreground" data-testid="vm-lock-unlock-command">
				{m['vms.lock.unlockCommand']({ vmid: String(store.entity.vmid) })}
			</p>
		{/if}

		<div class="mt-5">
			<VmActionBar onDelete={() => { deleteOpen = true; }} />
		</div>
	</header>

	<div class="mt-6 grid grid-cols-2 gap-4 md:grid-cols-4">
		<StatCard
			label={m['vms.detail.statCpu']()}
			value={store.entity.cpuCores}
			hint={m['common.coreCount']({ count: store.entity.cpuCores })}
			data-testid="vm-stat-cpu"
		/>
		<StatCard
			label={m['vms.detail.statMemory']()}
			value={formatBytes(store.entity.memoryTotal)}
			data-testid="vm-stat-memory"
		/>
		<StatCard
			label={m['vms.detail.statDisk']()}
			value={formatBytes(store.entity.diskTotal)}
			data-testid="vm-stat-disk"
		/>
		<StatCard
			label={m['vms.detail.statUptime']()}
			value={store.entity.uptimeSeconds ? formatUptime(store.entity.uptimeSeconds) : m['common.dash']()}
			data-testid="vm-stat-uptime"
		/>
	</div>

	<VmMetricsRow />

	<div class="mt-8">
		<Tabs {tabs} bind:active={activeTab} look="underline" />

		<div
			id="panel-overview"
			role="tabpanel"
			aria-labelledby="tab-overview"
			hidden={activeTab !== 'overview'}
			class="mt-4 rounded-xl border border-border bg-card p-6 shadow-card"
		>
			<section>
				<div class="flex items-center justify-between">
	<h2 class="text-sm font-medium text-muted-foreground">{m['vms.detail.descriptionLabel']()}</h2>
	{#if !editingDescription}
		<Button variant="secondary" size="sm" onclick={startEditDescription} label={m['common.edit']()}>
			{m['common.edit']()}
		</Button>
	{/if}
</div>
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
						use:focusOnMount
					></textarea>
					<div class="mt-3 flex gap-2">
						<Button size="sm" onclick={() => void commitDescription()} data-testid="vm-description-save">
							{m['common.save']()}
						</Button>
						<Button variant="secondary" size="sm" onclick={cancelDescription}>
							{m['common.cancel']()}
						</Button>
					</div>
				{:else}
					<div
						class="mt-3 w-full rounded-lg border border-dashed border-border bg-muted/30 px-4 py-4 text-left text-sm leading-6 hover:cursor-text hover:bg-muted/50"
						role="button"
						tabindex="0"
						onclick={startEditDescription}
						onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); startEditDescription(); } }}
						title={m['vms.detail.clickToEdit']()}
						data-testid="vm-description"
					>
						{#if store.entity.descriptionHtml}
							<article class="prose prose-sm max-w-none dark:prose-invert">
								<!-- eslint-disable-next-line svelte/no-at-html-tags -- backend renderer is XSS-safe (server/internal/httpapi/markdown.go) -->
								{@html store.entity.descriptionHtml}
							</article>
						{:else}
							{m['vms.detail.noDescription']()}
						{/if}
					</div>
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
		<Alert data-testid="vm-patch-error" class="mt-4">{store.patchError}</Alert>
	{/if}

	<DeleteVmDialog bind:open={deleteOpen} />
{/if}
