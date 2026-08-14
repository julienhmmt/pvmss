<script lang="ts">
	import { onMount } from 'svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { getVmDetailContext } from './detail.svelte';
	import CreateSnapshotDialog from './CreateSnapshotDialog.svelte';
	import DeleteSnapshotDialog from './DeleteSnapshotDialog.svelte';
	import RollbackSnapshotDialog from './RollbackSnapshotDialog.svelte';
	import { VmSnapshotsStore, type VmSnapshot } from './snapshots.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const vmStore = getVmDetailContext();
	const tray = getTaskTrayContext();
	const snapshots = new VmSnapshotsStore(vmStore.cluster, vmStore.vmid, tray);
	let createOpen = $state(false);
	let rollbackOpen = $state(false);
	let deleteOpen = $state(false);
	let selected = $state<VmSnapshot | null>(null);

	onMount(() => {
		void snapshots.load();
		return tray.onTaskOk(() => {
			void snapshots.load();
			void vmStore.load();
		});
	});

	function openRollback(snapshot: VmSnapshot): void {
		selected = snapshot;
		rollbackOpen = true;
	}

	function openDelete(snapshot: VmSnapshot): void {
		selected = snapshot;
		deleteOpen = true;
	}

	function formatCreatedAt(value: string): string {
		return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
	}
</script>

<section aria-labelledby="snapshots-heading" data-testid="vm-snapshots">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h2 id="snapshots-heading" class="text-lg font-semibold">{m['vms.snapshots.heading']()}</h2>
			<p class="mt-1 text-sm text-muted-foreground">{m['vms.snapshots.description']()}</p>
		</div>
		<div class="flex items-center gap-3">
			<span class="rounded-full bg-muted px-3 py-1 text-sm font-medium" data-testid="snapshot-counter">{snapshots.snapshots.length}/{snapshots.maxSnapshots ?? m['common.dash']()}</span>
			<button type="button" class="rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90" onclick={() => (createOpen = true)} data-testid="snapshot-create-open">{m['vms.snapshots.createButton']()}</button>
		</div>
	</div>
	{#if snapshots.maxSnapshots !== null && snapshots.snapshots.length >= snapshots.maxSnapshots}
		<p role="status" class="mt-3 text-sm text-muted-foreground" data-testid="snapshot-limit-message">{m['vms.snapshots.limitMessage']()}</p>
	{/if}

	{#if snapshots.loading && snapshots.snapshots.length === 0}
		<p role="status" aria-live="polite" class="mt-6 text-sm text-muted-foreground">{m['vms.snapshots.loading']()}</p>
	{:else if snapshots.error && snapshots.snapshots.length === 0}
		<p role="alert" class="mt-6 text-sm text-destructive" data-testid="snapshot-list-error">{snapshots.error}</p>
	{:else if snapshots.snapshots.length === 0}
		<p class="mt-6 rounded-md border border-dashed border-border p-6 text-center text-sm text-muted-foreground" data-testid="snapshot-empty">{m['vms.snapshots.empty']()}</p>
	{:else}
		<ul class="mt-6 space-y-3" aria-label={m['vms.snapshots.listLabel']()}>
			{#each snapshots.snapshots as snapshot (snapshot.name)}
				<li class="flex flex-wrap items-center justify-between gap-3 rounded-md border border-border p-4" data-testid="snapshot-row">
					<div class="min-w-0">
						<div class="flex flex-wrap items-center gap-2">
							<strong class="text-sm">{snapshot.name}</strong>
							{#if snapshot.vmstate}<span class="rounded-full bg-primary/10 px-2 py-0.5 text-xs text-primary">{m['vms.snapshots.ramState']()}</span>{/if}
						</div>
						<p class="mt-1 text-xs text-muted-foreground">{formatCreatedAt(snapshot.createdAt)}</p>
						{#if snapshot.description}<p class="mt-2 text-sm">{snapshot.description}</p>{/if}
					</div>
					<div class="flex shrink-0 gap-2">
						<button type="button" class="rounded-md border border-border px-3 py-1.5 text-sm hover:bg-muted" onclick={() => openRollback(snapshot)} data-testid="snapshot-rollback-open">{m['vms.snapshots.restore']()}</button>
						<button type="button" class="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-1.5 text-sm text-destructive hover:bg-destructive/10" onclick={() => openDelete(snapshot)} data-testid="snapshot-delete-open">{m['vms.snapshots.delete']()}</button>
					</div>
				</li>
			{/each}
		</ul>
	{/if}

	<CreateSnapshotDialog bind:open={createOpen} store={snapshots} status={vmStore.entity?.status ?? 'stopped'} />
	<RollbackSnapshotDialog bind:open={rollbackOpen} store={snapshots} snapshot={selected} />
	<DeleteSnapshotDialog bind:open={deleteOpen} store={snapshots} snapshot={selected} />
</section>
