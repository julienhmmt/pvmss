<script lang="ts">
	import { onMount } from 'svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { getVmDetailContext } from './detail.svelte';
	import CreateSnapshotDialog from './CreateSnapshotDialog.svelte';
	import DeleteSnapshotDialog from './DeleteSnapshotDialog.svelte';
	import RollbackSnapshotDialog from './RollbackSnapshotDialog.svelte';
	import { VmSnapshotsStore, type VmSnapshot } from './snapshots.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';

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

<Card pad="lg" aria-labelledby="snapshots-heading" data-testid="vm-snapshots">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h2 id="snapshots-heading" class="text-lg font-semibold">{m['vms.snapshots.heading']()}</h2>
			<p class="mt-1 text-sm text-muted-foreground">{m['vms.snapshots.description']()}</p>
		</div>
		<div class="flex items-center gap-3">
			<span class="rounded-full bg-muted px-3 py-1 text-sm font-medium" data-testid="snapshot-counter">{snapshots.snapshots.length}/{snapshots.maxSnapshots ? snapshots.maxSnapshots : m['common.dash']()}</span>
			<Button onclick={() => (createOpen = true)} data-testid="snapshot-create-open">{m['vms.snapshots.createButton']()}</Button>
		</div>
	</div>
	{#if snapshots.maxSnapshots !== null && snapshots.maxSnapshots > 0 && snapshots.snapshots.length >= snapshots.maxSnapshots}
		<p role="status" class="mt-3 text-sm text-muted-foreground" data-testid="snapshot-limit-message">{m['vms.snapshots.limitMessage']()}</p>
	{/if}

	{#if snapshots.loading && snapshots.snapshots.length === 0}
		<div role="status" aria-live="polite" class="mt-6">
			<span class="sr-only">{m['vms.snapshots.loading']()}</span>
			<TableSkeleton columns={3} />
		</div>
	{:else if snapshots.error && snapshots.snapshots.length === 0}
		<Alert data-testid="snapshot-list-error" class="mt-6">{snapshots.error}</Alert>
	{:else if snapshots.snapshots.length === 0}
		<EmptyState
			title={m['vms.snapshots.empty']()}
			dataTestid="snapshot-empty"
			class="mt-6 rounded-xl border border-dashed border-border"
		>
			{#snippet actions()}
				<Button onclick={() => (createOpen = true)}>{m['vms.snapshots.createButton']()}</Button>
			{/snippet}
		</EmptyState>
	{:else}
		<ul class="mt-6 divide-y divide-border" aria-label={m['vms.snapshots.listLabel']()}>
			{#each snapshots.snapshots as snapshot (snapshot.name)}
				<li class="flex flex-wrap items-center justify-between gap-3 py-4 first:pt-0 last:pb-0" data-testid="snapshot-row">
					<div class="min-w-0">
						<div class="flex flex-wrap items-center gap-2">
							<strong class="text-sm">{snapshot.name}</strong>
							{#if snapshot.vmstate}<Pill tone="accent" dot={false} label={m['vms.snapshots.ramState']()} />{/if}
						</div>
						<p class="mt-1 text-xs text-muted-foreground">{formatCreatedAt(snapshot.createdAt)}</p>
						{#if snapshot.description}<p class="mt-2 text-sm">{snapshot.description}</p>{/if}
					</div>
					<div class="flex shrink-0 gap-2">
						<Button variant="secondary" size="sm" onclick={() => openRollback(snapshot)} data-testid="snapshot-rollback-open" label={m['vms.snapshots.restore']()}>{m['vms.snapshots.restore']()}</Button>
						<Button variant="destructive" size="sm" onclick={() => openDelete(snapshot)} data-testid="snapshot-delete-open" label={m['vms.snapshots.delete']()}>{m['vms.snapshots.delete']()}</Button>
					</div>
				</li>
			{/each}
		</ul>
	{/if}

	<CreateSnapshotDialog bind:open={createOpen} store={snapshots} status={vmStore.entity?.status ?? 'stopped'} />
	<RollbackSnapshotDialog bind:open={rollbackOpen} store={snapshots} snapshot={selected} />
	<DeleteSnapshotDialog bind:open={deleteOpen} store={snapshots} snapshot={selected} />
</Card>
