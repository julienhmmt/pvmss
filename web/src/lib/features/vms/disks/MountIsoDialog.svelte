<script lang="ts">
	import { getVmDetailContext } from '../detail.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Button from '$lib/shared/ui/Button.svelte';

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
			<Select
				bind:value={selectedIso}
				options={[
					{ value: '', label: m['vms.disks.mountSelectIso']() },
					...(store.hardwareOptions?.isos ?? []).map((iso) => ({ value: iso.volId, label: iso.name }))
				]}
				data-testid="mount-iso-select"
			/>
		</label>
		{#if store.writeError}
			<Alert>{store.writeError}</Alert>
		{/if}
		<div class="mt-2 flex justify-end gap-2">
			<Button variant="secondary" onclick={close}>{m['common.cancel']()}</Button>
			<Button
				disabled={!selectedIso}
				loading={store.cdromInFlight}
				onclick={submit}
				data-testid="mount-iso-submit"
			>
				{store.cdromInFlight ? m['common.mounting']() : m['vms.disks.mount']()}
			</Button>
		</div>
	</div>
</Dialog>
