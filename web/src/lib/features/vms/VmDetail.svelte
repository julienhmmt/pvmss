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
	import VmMetricsRow from './VmMetricsRow.svelte';
	import VmConnectTab from './VmConnectTab.svelte';
	import VmStateBanners from './VmStateBanners.svelte';
	import MachineStatusPill from './MachineStatusPill.svelte';
	import { displayStatus } from './display-status';
	import { compactBytes, machineInitials, machineTone } from './machine-row';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { getTaskOutcomeLedgerContext } from '$lib/features/tasks/task-outcome-ledger.svelte';
	import { getPowerActionsContext } from '$lib/features/tasks/power-actions.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { resolve } from '$app/paths';
	import Button from '$lib/shared/ui/Button.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import OsMark from '$lib/shared/ui/OsMark.svelte';
	import ConfirmDialog from '$lib/shared/ui/ConfirmDialog.svelte';
	import { focusOnMount } from '$lib/shared/ui/focus-on-mount';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getVmDetailContext();
	const session = getSessionContext();
	const tray = getTaskTrayContext();
	const ledger = getTaskOutcomeLedgerContext();
	const powerActions = getPowerActionsContext();
	const toast = getToastContext();

	// Connection-first detail (DESIGN.md §6.3): Connect is the default tab,
	// Configuration holds the existing panels as sub-tabs, Activity is the
	// per-machine audit timeline.
	const tabs = [
		{ id: 'connect', label: () => m['vms.detail.tabConnect']() },
		{ id: 'configuration', label: () => m['vms.detail.tabConfiguration']() },
		{ id: 'activity', label: () => m['vm.activity.tab']() }
	];
	let activeTab = $state('connect');

	const configTabs = [
		{ id: 'overview', label: () => m['vms.detail.tabSummary']() },
		{ id: 'disks', label: () => m['vms.detail.tabDisks']() },
		{ id: 'network', label: () => m['vms.detail.tabNetwork']() },
		{ id: 'hardware', label: () => m['vms.detail.tabHardware']() },
		{ id: 'cloudinit', label: () => m['vms.detail.tabCloudinit']() },
		{ id: 'snapshots', label: () => m['vms.detail.tabSnapshots']() }
	];
	let configTab = $state('overview');

	const status = $derived(
		store.entity === null
			? 'stopped'
			: displayStatus(
					{ cluster: store.cluster, vmid: store.vmid, status: store.entity.status },
					{ tray: { tasks: tray.tasks }, ledger, inFlightAction: store.inFlightActionKind }
				)
	);

	// Mirror this page's in-flight power action into the app-wide registry so
	// the Activity screen and the sidebar count see it too.
	$effect(() => {
		const kind = store.inFlightActionKind;
		const name = store.entity?.name ?? '';
		if (kind !== null) powerActions.begin({ cluster: store.cluster, vmid: store.vmid, name, action: kind });
		else powerActions.end(store.cluster, store.vmid);
	});
	$effect(() => () => powerActions.end(store.cluster, store.vmid));

	let confirmingShutdown = $state(false);

	async function power(kind: 'start' | 'shutdown'): Promise<void> {
		confirmingShutdown = false;
		const name = store.entity?.name ?? '';
		await store.action(kind);
		if (store.actionError) {
			toast.error(m['toast.vmActionFailed']({ error: store.actionError }));
		} else {
			toast.success(kind === 'start' ? m['toast.vmStarted']({ name }) : m['toast.vmShutdown']({ name }));
		}
	}

	let deleteOpen = $state(false);
	let editingName = $state(false);
	let editingDescription = $state(false);
	let nameDraft = $state('');
	let descriptionDraft = $state('');
	let retrofitOpen = $state(false);

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
	{@const entity = store.entity}
	<header class="mb-6" data-testid="vm-detail-header">
		<a
			href={resolve('/vms')}
			class="pv-focus mb-4 inline-flex items-center gap-1 rounded text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
			data-testid="page-back-link"
		>
			<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-4 w-4" aria-hidden="true">
				<path d="M12 5l-5 5 5 5" />
			</svg>
			{m['vms.detail.back']()}
		</a>
		<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
			<div class="flex min-w-0 items-center gap-4">
				<span class="max-[369px]:hidden"><OsMark initials={machineInitials(entity.name)} tone={machineTone(entity.name)} size="lg" /></span>
				<div class="min-w-0">
					<h1 id="page-heading" tabindex="-1" class="flex flex-wrap items-center gap-2 text-2xl font-semibold tracking-tight focus:outline-none">
						{#if editingName}
							<input
								type="text"
								class="pv-input w-auto max-w-full text-2xl font-semibold tracking-tight"
								bind:value={nameDraft}
								onkeydown={handleNameKeydown}
								aria-label={m['vms.detail.clickToRename']()}
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
								class="truncate text-left hover:cursor-text hover:underline"
								onclick={startEditName}
								title={m['vms.detail.clickToRename']()}
								data-testid="vm-name"
							>
								{entity.name}
							</button>
						{/if}
					</h1>
					<p class="mt-1 text-sm text-muted-foreground">
						{m['vms.detail.identityLine']({ cpu: entity.cpuCores, memory: compactBytes(entity.memoryTotal), disk: compactBytes(entity.diskTotal) })}
						· <span class="font-mono">VM {entity.vmid}</span>
					</p>
					<p class="mt-0.5 font-mono text-xs text-muted-foreground-subtle" data-testid="vm-meta">
						{m['vms.detail.meta']({ vmid: String(entity.vmid), node: entity.node, pool: entity.pool })}
					</p>
				</div>
			</div>
			<div class="flex flex-wrap items-center gap-2">
				<span aria-live="polite" data-testid="vm-status" data-status={status}>
					<MachineStatusPill {status} size="md" pending={store.actionInFlight} />
				</span>
				{#if entity.lock}
					<span data-testid="vm-lock-badge">
						<Pill tone="warn" size="md" dot={false} label={m['vms.lock.badge']({ lock: entity.lock })} />
					</span>
				{/if}
				{#if entity.status === 'running'}
					<Button
						variant="secondary"
						loading={store.actionInFlight && store.inFlightActionKind === 'shutdown'}
						disabled={store.actionInFlight}
						onclick={() => (confirmingShutdown = true)}
						data-testid="vm-power-shutdown"
					>
						{m['vms.detail.power.shutdown']()}
					</Button>
				{:else if entity.status === 'stopped'}
					<Button
						loading={store.actionInFlight && store.inFlightActionKind === 'start'}
						disabled={store.actionInFlight || status === 'provisioning'}
						onclick={() => void power('start')}
						data-testid="vm-power-start"
					>
						{m['vms.detail.power.start']()}
					</Button>
				{/if}
			</div>
		</div>

		{#if entity.lock && session.isAdmin}
			<p class="mt-2 font-mono text-xs text-muted-foreground" data-testid="vm-lock-unlock-command">
				{m['vms.lock.unlockCommand']({ vmid: String(entity.vmid) })}
			</p>
		{/if}
	</header>

	<div class="mb-6 grid gap-3 empty:hidden">
		<VmStateBanners
			{status}
			{confirmingShutdown}
			onConfirmShutdown={() => void power('shutdown')}
			onCancelShutdown={() => (confirmingShutdown = false)}
			detail={ledger.detail(store.cluster, store.vmid)}
		/>
		{#if entity.baselineState === 'not_delivered'}
			<div class="rounded-xl border border-warning-soft-border bg-warning-soft p-4 text-warning-soft-foreground" data-testid="vm-baseline-not-delivered">
				<p class="text-sm font-medium">{m['vms.detail.baselineNotDelivered']()}</p>
				<p class="text-xs">{m['vms.detail.baselineNotDeliveredHint']()}</p>
				{#if entity.baselineError}
					<p class="mt-1 font-mono text-xs" data-testid="vm-baseline-error">{entity.baselineError}</p>
				{/if}
			</div>
		{/if}
	</div>

	<Tabs {tabs} bind:active={activeTab} look="underline" />

	<div id="panel-connect" role="tabpanel" aria-labelledby="tab-connect" hidden={activeTab !== 'connect'} class="mt-5">
		<VmConnectTab {status} />
	</div>

	<div id="panel-configuration" role="tabpanel" aria-labelledby="tab-configuration" hidden={activeTab !== 'configuration'} class="mt-5">
		<Tabs tabs={configTabs} bind:active={configTab} />

		<div
			id="panel-overview"
			role="tabpanel"
			aria-labelledby="tab-overview"
			hidden={configTab !== 'overview'}
			class="mt-4 grid gap-5"
		>
			<section class="rounded-xl border border-border bg-card p-6 shadow-card">
				<dl class="grid gap-x-6 gap-y-4 text-sm sm:grid-cols-2" data-testid="vm-config-summary">
					<div>
						<dt class="text-xs text-muted-foreground">{m['vms.detail.config.size']()}</dt>
						<dd class="mt-0.5">{m['vms.detail.config.sizeValue']({ cpu: entity.cpuCores, memory: compactBytes(entity.memoryTotal) })}</dd>
					</div>
					<div>
						<dt class="text-xs text-muted-foreground">{m['vms.detail.config.disk']()}</dt>
						<dd class="mt-0.5 font-mono">{compactBytes(entity.diskTotal)}</dd>
					</div>
					<div>
						<dt class="text-xs text-muted-foreground">{m['vms.detail.config.placement']()}</dt>
						<dd class="mt-0.5">{m['vms.detail.config.managed']()} <span class="font-mono text-xs text-muted-foreground">({entity.node})</span></dd>
					</div>
					<div>
						<dt class="text-xs text-muted-foreground">{m['vms.detail.config.identifier']()}</dt>
						<dd class="mt-0.5 font-mono">{entity.cluster} / {entity.vmid}</dd>
					</div>
				</dl>
				{#if entity.baselineState === 'applied'}
					<p class="mt-4 text-sm text-muted-foreground" data-testid="vm-baseline-applied">{m['vms.detail.baselineApplied']()}</p>
				{:else if entity.baselineState === 'override'}
					<p class="mt-4 text-sm text-muted-foreground" data-testid="vm-baseline-override">{m['vms.detail.baselineOverride']()}</p>
				{/if}
			</section>

			<section class="rounded-xl border border-border bg-card p-6 shadow-card">
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
						{#if entity.descriptionHtml}
							<article class="prose prose-sm max-w-none dark:prose-invert">
								<!-- eslint-disable-next-line svelte/no-at-html-tags -- backend renderer is XSS-safe (server/internal/httpapi/markdown.go) -->
								{@html entity.descriptionHtml}
							</article>
						{:else}
							{m['vms.detail.noDescription']()}
						{/if}
					</div>
				{/if}
			</section>

			<section class="rounded-xl border border-border bg-card p-6 shadow-card" aria-labelledby="vm-power-heading">
				<h2 id="vm-power-heading" class="mb-4 text-sm font-medium text-muted-foreground">{m['vms.detail.config.power']()}</h2>
				<VmActionBar onDelete={() => { deleteOpen = true; }} />
				{#if session.isAdmin}
					<div class="mt-4 flex flex-wrap items-center gap-2" data-testid="vm-admin-actions">
						<Button
							size="sm"
							variant="secondary"
							disabled={store.retrofitInFlight}
							onclick={() => { retrofitOpen = true; }}
							data-testid="vm-retrofit-seabios-btn"
							title={m['vms.detail.retrofitSeabiosHint']()}
							label={m['vms.detail.retrofitSeabios']()}
						>
							{m['vms.detail.retrofitSeabios']()}
						</Button>
						{#if store.retrofitError}
							<p class="text-destructive text-sm" data-testid="vm-retrofit-seabios-error">{store.retrofitError}</p>
						{/if}
					</div>
				{/if}
			</section>

			<section aria-labelledby="vm-usage-heading">
				<h2 id="vm-usage-heading" class="sr-only">{m['vms.detail.config.usage']()}</h2>
				<VmMetricsRow />
			</section>
		</div>

		<div id="panel-disks" role="tabpanel" aria-labelledby="tab-disks" hidden={configTab !== 'disks'} class="mt-4">
			<VmDisksTab />
		</div>

		<div id="panel-network" role="tabpanel" aria-labelledby="tab-network" hidden={configTab !== 'network'} class="mt-4">
			<VmNetworkTab />
		</div>

		<div id="panel-hardware" role="tabpanel" aria-labelledby="tab-hardware" hidden={configTab !== 'hardware'} class="mt-4">
			<VmHardwareTab />
		</div>

		<div id="panel-cloudinit" role="tabpanel" aria-labelledby="tab-cloudinit" hidden={configTab !== 'cloudinit'} class="mt-4">
			{#if activeTab === 'configuration' && configTab === 'cloudinit'}
				<CloudInitTab />
			{/if}
		</div>

		<div id="panel-snapshots" role="tabpanel" aria-labelledby="tab-snapshots" hidden={configTab !== 'snapshots'} class="mt-4">
			{#if activeTab === 'configuration' && configTab === 'snapshots'}
				<VmSnapshotsTab />
			{/if}
		</div>
	</div>

	<div id="panel-activity" role="tabpanel" aria-labelledby="tab-activity" hidden={activeTab !== 'activity'} class="mt-5">
		{#if activeTab === 'activity'}
			<VmActivityTab />
		{/if}
	</div>

	{#if store.patchError}
		<Alert data-testid="vm-patch-error" class="mt-4">{store.patchError}</Alert>
	{/if}

	<DeleteVmDialog bind:open={deleteOpen} />

	<ConfirmDialog
		open={retrofitOpen}
		title={m['vms.detail.retrofitSeabiosConfirmTitle']({ name: store.entity?.name ?? '' })}
		message={m['vms.detail.retrofitSeabiosConfirmMessage']()}
		confirmLabel={m['vms.detail.retrofitSeabiosConfirm']()}
		cancelLabel={m['common.cancel']()}
		confirming={store.retrofitInFlight}
		testId="vm-retrofit-seabios-confirm"
		onConfirm={async () => { await store.retrofitSeaBIOS(true); if (!store.retrofitError) retrofitOpen = false; }}
		onClose={() => { retrofitOpen = false; }}
	/>
{/if}
