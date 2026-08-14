<script lang="ts">
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import type { VmStatus } from './list.svelte';
	import { VmSnapshotsStore } from './snapshots.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		open?: boolean;
		store: VmSnapshotsStore;
		status: VmStatus;
	}

	let { open = $bindable(false), store, status }: Props = $props();
	let name = $state('');
	let description = $state('');
	let vmstate = $state(false);
	let running = $derived(status === 'running');

	async function submit(): Promise<void> {
		const created = await store.create(name.trim(), description.trim(), vmstate);
		if (!created) return;
		name = '';
		description = '';
		vmstate = false;
		open = false;
	}

	function close(): void {
		store.clearError();
		open = false;
	}
</script>

<Dialog bind:open labelledBy="create-snapshot-title" onClose={close}>
	<h2 id="create-snapshot-title" class="mb-2 text-lg font-semibold">{m['vms.snapshots.createDialogTitle']()}</h2>
	<p class="mb-4 text-sm text-muted-foreground">{m['vms.snapshots.createDescription']()}</p>
	<form onsubmit={(event) => { event.preventDefault(); void submit(); }}>
		<div class="space-y-4">
			<label class="block text-sm font-medium" for="snapshot-name">
				{m['vms.snapshots.createName']()}
				<input id="snapshot-name" class="mt-1 w-full rounded-md border border-border bg-background px-3 py-2" bind:value={name} maxlength="40" required pattern="[A-Za-z0-9_][A-Za-z0-9_.-]*" data-testid="snapshot-name" />
			</label>
			<label class="block text-sm font-medium" for="snapshot-description">
				{m['vms.snapshots.createDescriptionLabel']()} <span class="font-normal text-muted-foreground">{m['common.optional']()}</span>
				<textarea id="snapshot-description" class="mt-1 w-full rounded-md border border-border bg-background px-3 py-2" bind:value={description} rows="3" data-testid="snapshot-description"></textarea>
			</label>
			<label class="flex items-start gap-2 text-sm">
				<input class="mt-0.5 size-4 accent-primary" type="checkbox" bind:checked={vmstate} disabled={!running} data-testid="snapshot-vmstate" />
				<span>
					{m['vms.snapshots.createRamState']()}
					{#if !running}<span class="block text-xs font-normal text-muted-foreground">{m['vms.snapshots.createRamHint']()}</span>{/if}
				</span>
			</label>
		</div>
		{#if store.error}<p role="alert" class="mt-4 text-sm text-destructive" data-testid="snapshot-create-error">{store.error}</p>{/if}
		<div class="mt-6 flex justify-end gap-2">
			<button type="button" class="rounded-md border border-border px-4 py-2 text-sm hover:bg-muted" onclick={close}>{m['common.cancel']()}</button>
			<button type="submit" class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50" disabled={store.inFlight || name.trim() === ''} data-testid="snapshot-create-confirm">
				{store.inFlight ? m['common.creating']() : m['vms.snapshots.createButton']()}
			</button>
		</div>
	</form>
</Dialog>
