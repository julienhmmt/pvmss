<script lang="ts">
	import { slide } from 'svelte/transition';
	import { flip } from 'svelte/animate';
	import { t } from 'svelte-i18n';
	import { Camera, ClockCounterClockwise, Trash } from 'phosphor-svelte';
	import type { SnapshotList } from '$lib/api/vm-details';

	interface Props {
		snapshotData: SnapshotList | null;
		creatingSnapshot: boolean;
		showSnapshotForm: boolean;
		snapName: string;
		snapDesc: string;
		onToggleForm: () => void;
		onCreateSnapshot: () => void;
		onDeleteSnapshot: (name: string) => void;
		onRollback: (name: string) => void;
		onSnapNameChange: (value: string) => void;
		onSnapDescChange: (value: string) => void;
	}

	let {
		snapshotData,
		creatingSnapshot,
		showSnapshotForm,
		snapName,
		snapDesc,
		onToggleForm,
		onCreateSnapshot,
		onDeleteSnapshot,
		onRollback,
		onSnapNameChange,
		onSnapDescChange
	}: Props = $props();

	function snapshotDate(ts: number): string {
		if (!ts) return '—';
		return new Date(ts * 1000).toLocaleString();
	}

	const nonCurrentSnapshots = $derived(
		snapshotData?.snapshots.filter((s) => !s.current) ?? []
	);
	const canCreate = $derived(
		snapshotData && nonCurrentSnapshots.length < snapshotData.maxAllowed
	);
</script>

<div class="pv-table-wrap">
	<div class="flex items-center justify-between border-b border-border px-4 py-3">
		<span class="text-sm font-medium">
			{snapshotData
				? `${nonCurrentSnapshots.length} / ${snapshotData.maxAllowed} ${$t('vm.snapshots')}`
				: $t('vm.snapshots')}
		</span>
		{#if !showSnapshotForm && canCreate}
			<button
				class="inline-flex items-center gap-1 text-sm text-primary hover:underline"
				onclick={onToggleForm}
			>
				<Camera class="h-4 w-4" />
				{$t('vm.createSnapshot')}
			</button>
		{/if}
	</div>

	{#if showSnapshotForm}
		<div transition:slide={{ duration: 200 }} class="border-b border-border px-4 py-3">
			<div class="mb-2 flex gap-2">
				<input
					class="flex-1 rounded border border-border bg-background px-2 py-1 text-sm"
					placeholder={$t('vm.snapshotNamePlaceholder')}
					bind:value={snapName}
					oninput={(e: Event) => onSnapNameChange((e.currentTarget as HTMLInputElement).value)}
				/>
				<input
					class="flex-1 rounded border border-border bg-background px-2 py-1 text-sm"
					placeholder={$t('common.description')}
					bind:value={snapDesc}
					oninput={(e: Event) => onSnapDescChange((e.currentTarget as HTMLInputElement).value)}
				/>
			</div>
			<div class="flex gap-2">
				<button
					class="pv-btn-primary text-xs"
					onclick={onCreateSnapshot}
					disabled={creatingSnapshot || !snapName.trim()}
				>
					{creatingSnapshot ? $t('common.saving') : $t('common.create')}
				</button>
				<button
					class="text-xs text-muted-foreground hover:text-foreground"
					onclick={onToggleForm}
				>
					{$t('common.cancel')}
				</button>
			</div>
		</div>
	{/if}

	{#if !snapshotData || nonCurrentSnapshots.length === 0}
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
				{#each nonCurrentSnapshots as snap (snap.name)}
					<tr animate:flip={{ duration: 200 }} class="pv-row">
						<td class="pv-td-mono">{snap.name}</td>
						<td class="text-sm text-muted-foreground">{snap.description || '—'}</td>
						<td class="pv-td-mono text-xs">{snapshotDate(snap.snaptime)}</td>
						<td>
							<div class="flex items-center gap-1">
								<button
									class="pv-action-btn"
									onclick={() => onRollback(snap.name)}
									title={$t('vm.rollback')}
								>
									<ClockCounterClockwise class="h-3.5 w-3.5" />
								</button>
								<button
									class="pv-action-btn pv-action-btn--stop"
									onclick={() => onDeleteSnapshot(snap.name)}
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
