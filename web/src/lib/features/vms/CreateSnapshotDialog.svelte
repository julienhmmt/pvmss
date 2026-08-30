<script lang="ts">
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Textarea from '$lib/shared/ui/Textarea.svelte';
	import Checkbox from '$lib/shared/ui/Checkbox.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
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

	// Capability preflight (ticket 07): the snapshot list response may carry a
	// `capability` block computed server-side from the VM's real config. When
	// it is absent (older server), fall back to the historical `!running` rule.
	const capability = $derived(store.capability);
	const canVMState = $derived(capability === null ? running : capability.canVMState);
	const canSnapshot = $derived(capability === null ? true : capability.canSnapshot);
	const vmStateReason = $derived(
		!running ? m['vms.snapshots.createRamHint']() : (capability?.warnings[0] ?? '')
	);
	const snapshotBlockedReason = $derived(
		capability !== null && !capability.canSnapshot ? (capability.warnings[0] ?? '') : ''
	);

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
			<FormField label={m['vms.snapshots.createName']()} hint={m['vms.snapshots.createNameHint']()} required error={store.error}>
				{#snippet children({ id, describedBy, invalid })}
					<TextField
						{id}
						{describedBy}
						{invalid}
						bind:value={name}
						maxLength={40}
						required
						pattern={ '[A-Za-z][A-Za-z0-9_-]{1,39}' }
						data-testid="snapshot-name"
					/>
				{/snippet}
			</FormField>
			<FormField label={m['vms.snapshots.createDescriptionLabel']()} hint={m['common.optional']()}>
				{#snippet children({ id, describedBy, invalid })}
					<Textarea
						{id}
						{describedBy}
						{invalid}
						bind:value={description}
						rows={3}
						data-testid="snapshot-description"
					/>
				{/snippet}
			</FormField>
			<Checkbox
				label={m['vms.snapshots.createRamState']()}
				hint={vmStateReason}
				checked={vmstate}
				onToggle={(checked) => (vmstate = checked)}
				disabled={!canVMState}
			/>
			{#if snapshotBlockedReason}
				<p role="status" class="text-sm text-muted-foreground" data-testid="snapshot-blocked-reason">{snapshotBlockedReason}</p>
			{/if}
		</div>
		<div class="mt-6 flex justify-end gap-2">
			<Button type="button" variant="ghost" onclick={close}>{m['common.cancel']()}</Button>
			<Button
				type="submit"
				loading={store.inFlight}
				disabled={name.trim() === '' || !canSnapshot}
				data-testid="snapshot-create-confirm"
			>
				{store.inFlight ? m['common.creating']() : m['vms.snapshots.createButton']()}
			</Button>
		</div>
	</form>
</Dialog>
