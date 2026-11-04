<script lang="ts">
	import { onMount } from 'svelte';
	import { getTaskTrayContext } from '#lib/features/tasks/tasks.svelte.js';
	import { getVmDetailContext } from './detail.svelte';
	import CreateSnapshotDialog from './CreateSnapshotDialog.svelte';
	import DeleteSnapshotDialog from './DeleteSnapshotDialog.svelte';
	import RollbackSnapshotDialog from './RollbackSnapshotDialog.svelte';
	import { AUTO_SNAPSHOT_PREFIX, VmSnapshotsStore, type SnapshotFrequency, type VmSnapshot, type VmSnapshotSchedule } from './snapshots.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { formatDateTime } from '#lib/shared/format-date.js';
	import { getToastContext } from '#lib/shared/ui/toast.svelte.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import EmptyState from '#lib/shared/ui/EmptyState.svelte';
	import Pill from '#lib/shared/ui/Pill.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import TableSkeleton from '#lib/shared/ui/TableSkeleton.svelte';

	const SCHEDULE_REFRESH_MS = 60_000;

	const vmStore = getVmDetailContext();
	const tray = getTaskTrayContext();
	const toast = getToastContext();
	const snapshots = new VmSnapshotsStore(vmStore.cluster, vmStore.vmid, tray);
	let createOpen = $state(false);
	let rollbackOpen = $state(false);
	let deleteOpen = $state(false);
	let selected = $state<VmSnapshot | null>(null);

	/** Picks up what the 15-minute server sweep did while the page stays open. */
	async function refreshSchedule(): Promise<void> {
		const before = snapshots.schedule;
		if (!before || document.hidden) return;
		await snapshots.load(true);
		const after = snapshots.schedule;
		if (!after) return;
		if (after.lastRunAt !== before.lastRunAt) toast.success(m['toast.snapshotAutoTaken']());
		// A repeating failure toasts once; the alert in the form stays.
		if (after.lastFailure && after.lastFailure !== before.lastFailure) toast.error(failureMessage(after));
	}

	function failureMessage(schedule: VmSnapshotSchedule): string {
		const date = formatCreatedAt(schedule.lastFailureAt ?? '');
		switch (schedule.lastFailure) {
			case 'cap_reached':
				return m['vms.schedule.failedCap']({ date });
			case 'cluster_unavailable':
				return m['vms.schedule.failedUnavailable']({ date });
			case 'prune_failed':
				return m['vms.schedule.failedPrune']({ date });
			default:
				return m['vms.schedule.failedCreate']({ date });
		}
	}

	onMount(() => {
		void snapshots.load();
		const timer = setInterval(() => void refreshSchedule(), SCHEDULE_REFRESH_MS);
		const stopTaskListener = tray.onTaskOk(() => {
			void snapshots.load();
			void vmStore.load();
		});
		return () => {
			clearInterval(timer);
			stopTaskListener();
		};
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
		return formatDateTime(value);
	}

	// The form mirrors the stored values, not the object: a background refresh
	// returns a new object and must not wipe what the user is typing.
	const storedFrequency = $derived(snapshots.schedule?.frequency ?? 'off');
	const storedRetain = $derived(snapshots.schedule?.retain ?? 3);
	let scheduleFrequency = $state<SnapshotFrequency>('off');
	let scheduleRetain = $state(3);
	$effect(() => {
		scheduleFrequency = storedFrequency;
		scheduleRetain = storedRetain;
	});

	const retainLimit = $derived(snapshots.maxSnapshots && snapshots.maxSnapshots > 0 ? snapshots.maxSnapshots - 1 : null);
	const retainValid = $derived(scheduleRetain >= 1 && (retainLimit === null || scheduleRetain <= retainLimit));

	async function saveSchedule(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		if (scheduleFrequency === 'off') {
			if (await snapshots.clearSchedule()) toast.success(m['toast.snapshotScheduleCleared']());
			return;
		}
		if (!retainValid) return;
		if (await snapshots.saveSchedule(scheduleFrequency, scheduleRetain)) toast.success(m['toast.snapshotScheduleSaved']());
	}

	function isAutoSnapshot(snapshot: VmSnapshot): boolean {
		return snapshot.name.startsWith(AUTO_SNAPSHOT_PREFIX);
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

	<form class="mt-6 rounded-xl border border-border p-4" onsubmit={saveSchedule} data-testid="snapshot-schedule-form">
		<h3 class="text-sm font-semibold">{m['vms.schedule.heading']()}</h3>
		<p class="mt-1 text-xs text-muted-foreground">{m['vms.schedule.description']()}</p>
		<div class="mt-3 flex flex-wrap items-end gap-4">
			<fieldset class="flex flex-wrap gap-3 text-sm">
				<label class="flex items-center gap-1.5"><input type="radio" name="snapshot-frequency" value="off" bind:group={scheduleFrequency} data-testid="schedule-off" />{m['vms.schedule.off']()}</label>
				<label class="flex items-center gap-1.5"><input type="radio" name="snapshot-frequency" value="daily" bind:group={scheduleFrequency} data-testid="schedule-daily" />{m['vms.schedule.daily']()}</label>
				<label class="flex items-center gap-1.5"><input type="radio" name="snapshot-frequency" value="weekly" bind:group={scheduleFrequency} data-testid="schedule-weekly" />{m['vms.schedule.weekly']()}</label>
			</fieldset>
			<label class="flex flex-col gap-1 text-sm">
				<span>{m['vms.schedule.retainLabel']()}</span>
				<input type="number" min="1" max={retainLimit ?? undefined} bind:value={scheduleRetain} disabled={scheduleFrequency === 'off'} class="w-24 rounded-md border border-border bg-background px-2 py-1" data-testid="schedule-retain" />
			</label>
			<Button type="submit" disabled={snapshots.scheduleSaving || (scheduleFrequency !== 'off' && !retainValid)} data-testid="schedule-save">
				{snapshots.scheduleSaving ? m['vms.schedule.saving']() : m['vms.schedule.save']()}
			</Button>
		</div>
		<p class="mt-2 text-xs text-muted-foreground">{m['vms.schedule.retainHint']()}</p>
		{#if snapshots.schedule}
			<p class="mt-2 text-xs text-muted-foreground" data-testid="schedule-last-run">
				{snapshots.schedule.lastRunAt ? m['vms.schedule.lastRun']({ date: formatCreatedAt(snapshots.schedule.lastRunAt) }) : m['vms.schedule.neverRun']()}
			</p>
		{/if}
		{#if snapshots.schedule?.lastFailure}
			<Alert class="mt-3" data-testid="schedule-failure">{failureMessage(snapshots.schedule)}</Alert>
		{/if}
		{#if snapshots.scheduleError}
			<Alert class="mt-3" data-testid="schedule-error">{snapshots.scheduleError}</Alert>
		{/if}
	</form>

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
							{#if isAutoSnapshot(snapshot)}<Pill tone="accent" dot={false} label={m['vms.snapshots.autoBadge']()} data-testid="snapshot-auto-badge" />{/if}
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
