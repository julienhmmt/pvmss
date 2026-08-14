<script lang="ts">
	import { getVmDetailContext } from '../detail.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getVmDetailContext();

	interface Props {
		open?: boolean;
	}

	let { open = $bindable(false) }: Props = $props();
	let selectedIso = $state('');

	function close(): void {
		open = false;
		selectedIso = '';
	}

	async function submit(): Promise<void> {
		if (!selectedIso) return;
		await store.setCdrom('mount', selectedIso);
		close();
	}
</script>

<Dialog bind:open labelledBy="mount-iso-title" onClose={close}>
	<h2 id="mount-iso-title" class="mb-4 text-lg font-semibold">{m['vms.disks.mountDialogTitle']()}</h2>
	<div class="grid gap-3">
		<label class="grid gap-1 text-sm">
			{m['vms.disks.mountApprovedIso']()}
			<select
				class="rounded-md border border-border bg-background px-2 py-2"
				bind:value={selectedIso}
				data-testid="mount-iso-select"
			>
				<option value="">{m['vms.disks.mountSelectIso']()}</option>
				{#each store.hardwareOptions?.isos ?? [] as iso (iso.volId)}
					<option value={iso.volId}>{iso.name}</option>
				{/each}
			</select>
		</label>
		{#if store.writeError}
			<p role="alert" class="text-sm text-destructive">{store.writeError}</p>
		{/if}
		<div class="mt-2 flex justify-end gap-2">
			<button
				type="button"
				class="rounded-md border border-border px-4 py-2 text-sm hover:bg-muted"
				onclick={close}
			>
				Cancel
			</button>
			<button
				type="button"
				class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
				disabled={!selectedIso || store.cdromInFlight}
				onclick={submit}
				data-testid="mount-iso-submit"
			>
				{store.cdromInFlight ? m['common.mounting']() : m['vms.disks.mount']()}
			</button>
		</div>
	</div>
</Dialog>
