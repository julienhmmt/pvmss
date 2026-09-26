<script lang="ts">
	/**
	 * Activity (DESIGN.md §6.4): what is running right now, and what happened
	 * recently, across the user's machines. "In progress" reads the task tray
	 * (creations, snapshot work) and the power-action registry (starts,
	 * shutdowns); "Recent updates" is the merged audit trail of the user's
	 * machines. Every row links to the machine it is about.
	 */
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { getTaskTrayContext, type TaskKind } from '$lib/features/tasks/tasks.svelte';
	import { getPowerActionsContext } from '$lib/features/tasks/power-actions.svelte';
	import { ActivityStore, activityMessage } from '$lib/features/activity/activity.svelte';
	import type { VmAction } from '$lib/features/vms/detail.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import ButtonLink from '$lib/shared/ui/ButtonLink.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const tray = getTaskTrayContext();
	const powerActions = getPowerActionsContext();
	const store = new ActivityStore();

	onMount(() => {
		void store.load();
		// A finished task is news: refresh the timeline when one lands.
		return tray.onTaskOk(() => void store.load());
	});

	const TASK_LABELS: Record<TaskKind, () => string> = {
		vm_create: () => m['activity.kind.vm_create'](),
		vm_snapshot_create: () => m['activity.kind.vm_snapshot_create'](),
		vm_snapshot_rollback: () => m['activity.kind.vm_snapshot_rollback'](),
		vm_snapshot_delete: () => m['activity.kind.vm_snapshot_delete']()
	};

	const POWER_LABELS: Record<VmAction, () => string> = {
		start: () => m['activity.kind.start'](),
		stop: () => m['activity.kind.stop'](),
		shutdown: () => m['activity.kind.shutdown'](),
		reboot: () => m['activity.kind.reboot'](),
		reset: () => m['activity.kind.reset'](),
		pause: () => m['activity.kind.pause'](),
		resume: () => m['activity.kind.resume']()
	};

	interface InFlightRow {
		key: string;
		cluster: string;
		vmid: number;
		name: string;
		label: string;
	}

	const inFlight = $derived<InFlightRow[]>([
		...tray.tasks.map((task) => ({
			key: `task:${task.upid}`,
			cluster: task.cluster,
			vmid: task.vmid,
			name: task.name,
			label: TASK_LABELS[task.kind]()
		})),
		...powerActions.entries.map((entry) => ({
			key: `power:${entry.cluster}:${entry.vmid}`,
			cluster: entry.cluster,
			vmid: entry.vmid,
			name: entry.name,
			label: POWER_LABELS[entry.action]()
		}))
	]);

	function detailHref(cluster: string, vmid: number): string {
		return resolve('/vms/[cluster]/[vmid]', { cluster, vmid: String(vmid) });
	}

	const timeFormat = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
</script>

<svelte:head>
	<title>{m['activity.title']()}</title>
</svelte:head>

<section class="mx-auto w-full max-w-4xl">
	<PageHeader
		eyebrow={m['activity.eyebrow']()}
		title={m['activity.heading']()}
		description={m['activity.description']()}
		focusTarget
		divider={false}
	>
		{#snippet actions()}
			<span class="text-xs text-muted-foreground">{m['activity.window']()}</span>
		{/snippet}
	</PageHeader>

	{#if inFlight.length > 0}
		<section class="mb-6 rounded-xl border border-border bg-card shadow-card" aria-labelledby="activity-in-progress" data-testid="activity-in-progress">
			<h2 id="activity-in-progress" class="border-b border-border px-5 py-3 text-sm font-semibold">{m['activity.inProgress']()}</h2>
			<ul aria-live="polite">
				{#each inFlight as row (row.key)}
					<li class="border-b border-border-subtle last:border-b-0">
						<a
							href={detailHref(row.cluster, row.vmid)}
							class="pv-focus flex items-center gap-3 px-5 py-3 transition-colors hover:bg-muted/50"
							aria-label={m['activity.openMachine']({ name: row.name })}
							data-testid="activity-in-progress-row"
						>
							<span class="h-2 w-2 shrink-0 rounded-full bg-warning motion-safe:animate-pulse" aria-hidden="true"></span>
							<span class="font-medium">{row.name}</span>
							<span class="text-sm text-muted-foreground">{row.label}</span>
							<span class="ml-auto text-muted-foreground" aria-hidden="true">→</span>
						</a>
					</li>
				{/each}
			</ul>
		</section>
	{/if}

	<section class="rounded-xl border border-border bg-card shadow-card" aria-labelledby="activity-recent" data-testid="activity-recent">
		<h2 id="activity-recent" class="border-b border-border px-5 py-3 text-sm font-semibold">{m['activity.recent']()}</h2>
		{#if store.entries === null && store.loading}
			<div class="grid gap-3 p-5" role="status" aria-label={m['common.loading']()}>
				<Skeleton class="h-4 w-2/3" />
				<Skeleton class="h-4 w-1/2" />
				<Skeleton class="h-4 w-3/5" />
			</div>
		{:else if store.error}
			<EmptyState title={store.error} tone="error" dataTestid="activity-error">
				{#snippet actions()}
					<Button variant="secondary" onclick={() => void store.load()}>{m['activity.retry']()}</Button>
				{/snippet}
			</EmptyState>
		{:else if store.entries !== null && store.entries.length === 0}
			<EmptyState title={m['activity.emptyTitle']()} description={m['activity.emptyBody']()} dataTestid="activity-empty">
				{#snippet actions()}
					<ButtonLink href={resolve('/vms')} variant="secondary">{m['activity.goToMachines']()}</ButtonLink>
				{/snippet}
			</EmptyState>
		{:else if store.entries !== null}
			<ol>
				{#each store.entries as entry (entry.id)}
					<li class="border-b border-border-subtle last:border-b-0">
						<a
							href={detailHref(entry.cluster, entry.vmid)}
							class="pv-focus flex flex-wrap items-baseline gap-x-3 gap-y-0.5 px-5 py-3 transition-colors hover:bg-muted/50"
							data-testid="activity-row"
						>
							<span class="h-1.5 w-1.5 shrink-0 translate-y-[-2px] rounded-full bg-muted-foreground-subtle" aria-hidden="true"></span>
							<span class="font-medium">{entry.machineName}</span>
							<span class="text-sm text-muted-foreground">{activityMessage(entry.action)}</span>
							<span class="text-xs text-muted-foreground-subtle">{m['activity.by']({ actor: entry.actor })}</span>
							<time class="ml-auto text-xs tabular-nums text-muted-foreground" datetime={entry.timestamp}>{timeFormat.format(new Date(entry.timestamp))}</time>
						</a>
					</li>
				{/each}
			</ol>
		{/if}
	</section>
</section>
