<script lang="ts">
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import { VmSnapshotsStore } from './snapshots.svelte';
	import type { RollbackDiffEntry, VmSnapshot } from './snapshots.svelte';
	import { ApiRequestError } from '$lib/shared/api/client';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		open?: boolean;
		store: VmSnapshotsStore;
		snapshot: VmSnapshot | null;
	}

	let { open = $bindable(false), store, snapshot }: Props = $props();

	let diffOpen = $state(false);
	let diffEntries = $state<RollbackDiffEntry[] | null>(null);
	let diffLoading = $state(false);
	let diffError = $state<string | null>(null);

	async function confirm(): Promise<void> {
		if (snapshot === null) return;
		const rolledBack = await store.rollback(snapshot.name);
		if (rolledBack) open = false;
	}

	/** Toggles the "what will change" panel, fetching the config diff on first
	 *  open so the user can preview the rollback before committing to it. */
	async function toggleDiff(): Promise<void> {
		diffOpen = !diffOpen;
		if (!diffOpen || diffEntries !== null || snapshot === null) return;
		diffLoading = true;
		diffError = null;
		try {
			diffEntries = await store.rollbackDiff(snapshot.name);
		} catch (error: unknown) {
			diffError = error instanceof ApiRequestError ? error.message : m['vms.snapshots.errorOperation']();
		} finally {
			diffLoading = false;
		}
	}

	function close(): void {
		store.clearError();
		diffOpen = false;
		diffEntries = null;
		diffError = null;
		open = false;
	}
</script>

<Dialog bind:open labelledBy="rollback-snapshot-title" onClose={close}>
	<h2 id="rollback-snapshot-title" class="mb-4 text-lg font-semibold">{m['vms.snapshots.rollbackDialogTitle']()}</h2>
	<p class="mb-2 text-sm text-muted-foreground">
		{m['vms.snapshots.rollbackConfirm']({ name: snapshot?.name ?? '' })}
	</p>
	<p class="mb-4 text-sm font-medium text-destructive" data-testid="snapshot-rollback-restart-warning">
		{m['vms.snapshots.rollbackRestarts']()}
	</p>
	{#if store.error}<Alert data-testid="snapshot-rollback-error" class="mb-4">{store.error}</Alert>{/if}

	<div class="mb-4 rounded-lg border border-border">
		<button type="button" class="flex w-full items-center justify-between px-4 py-2.5 text-sm font-medium hover:bg-muted" onclick={() => void toggleDiff()} aria-expanded={diffOpen} data-testid="snapshot-rollback-diff-toggle">
			<span>{m['vms.snapshots.rollbackDiff']()}</span>
			<span aria-hidden="true">{diffOpen ? '−' : '+'}</span>
		</button>
		{#if diffOpen}
			<div class="border-t border-border px-4 py-3">
				{#if diffLoading}
					<p role="status" class="text-sm text-muted-foreground">{m['vms.snapshots.loading']()}</p>
				{:else if diffError}
					<Alert data-testid="snapshot-rollback-diff-error">{diffError}</Alert>
				{:else if diffEntries !== null && diffEntries.length === 0}
					<p role="status" class="text-sm text-muted-foreground" data-testid="snapshot-rollback-diff-empty">{m['vms.snapshots.rollbackDiffEmpty']()}</p>
				{:else if diffEntries !== null}
					<ul class="grid gap-1.5 text-sm" data-testid="snapshot-rollback-diff">
						{#each diffEntries as entry (entry.key)}
							<li class="grid grid-cols-[1fr_auto_auto] items-baseline gap-x-3 gap-y-0.5 border-b border-border-subtle pb-1.5 last:border-b-0 last:pb-0" data-testid="snapshot-rollback-diff-row">
								<code class="break-all text-muted-foreground">{entry.key}</code>
								<span class="text-right text-muted-foreground line-through decoration-destructive/60">{entry.before}</span>
								<span class="text-right text-foreground" aria-label={m['vms.snapshots.rollbackDiffBecomes']()}>→ {entry.after}</span>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
		{/if}
	</div>

	<div class="flex justify-end gap-2">
		<button type="button" class="rounded-md border border-border px-4 py-2 text-sm hover:bg-muted" onclick={close} data-testid="snapshot-rollback-cancel">{m['common.cancel']()}</button>
		<button type="button" class="rounded-md bg-destructive px-4 py-2 text-sm font-medium text-destructive-foreground hover:bg-destructive/90 disabled:opacity-50" disabled={store.inFlight || snapshot === null} onclick={() => void confirm()} data-testid="snapshot-rollback-confirm">
			{store.inFlight ? m['vms.snapshots.rollbackRestoring']() : m['vms.snapshots.rollbackButton']()}
		</button>
	</div>
</Dialog>
