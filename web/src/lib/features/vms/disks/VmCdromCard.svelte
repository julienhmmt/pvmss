<script lang="ts">
	import { getVmDetailContext } from '../detail.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import MountIsoDialog from './MountIsoDialog.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
	import DiscIcon from '$lib/shared/ui/icons/DiscIcon.svelte';
	import SpinnerIcon from '$lib/shared/ui/icons/SpinnerIcon.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Button from '$lib/shared/ui/Button.svelte';

	const store = getVmDetailContext();
	const toast = getToastContext();
	let mountOpen = $state(false);
	let bootConfirmOpen = $state(false);

	const isoMounted = $derived(store.entity?.cdrom?.state === 'mounted');
	// ide2 holds a cloud-init drive: mounting, ejecting or removing would destroy it.
	const occupied = $derived(store.entity?.cdrom?.state === 'occupied');

	async function bootFromCdrom(): Promise<void> {
		const vmName = store.entity?.name ?? '';
		// Optimistic feedback: the server request only returns once the guest
		// is up (it waits for the boot before restoring the boot order), so a
		// toast on response would land tens of seconds after the click. Fire
		// success now; a failure corrects it with an error toast.
		toast.success(m['toast.vmBootedFromCdrom']({ name: vmName }));
		const success = await store.bootFromCdrom();
		if (!success) {
			toast.error(store.bootCdromError ?? m['vms.detail.errorBootCdrom']());
		}
	}

	function requestBoot(): void {
		if (store.entity?.status === 'running') {
			bootConfirmOpen = true;
			return;
		}

		void bootFromCdrom();
	}
</script>

<Card as="div" pad="lg" data-testid="vm-cdrom">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h2 class="text-lg font-semibold">{m['vms.disks.cdromHeading']()}</h2>
			<p class="mt-1 text-sm text-muted-foreground" data-testid="vm-cdrom-state">
				{m['vms.disks.cdromState']()}
				<span class="font-medium text-foreground">{store.entity?.cdrom?.state ?? 'absent'}</span>
			</p>
			{#if store.entity?.cdrom?.isoVolId}
				<p class="mt-1 truncate font-mono text-xs text-muted-foreground">{store.entity.cdrom.isoVolId}</p>
			{/if}
		</div>
	</div>
	{#if occupied}
		<p class="mt-3 text-sm text-muted-foreground" data-testid="vm-cdrom-occupied">{m['vms.disks.cdromOccupied']()}</p>
	{/if}
	<div class="mt-5 flex flex-wrap gap-2">
		<Button
			disabled={store.cdromInFlight || occupied}
			onclick={() => (mountOpen = true)}
			data-testid="vm-cdrom-mount-open"
		>
			{m['vms.disks.mount']()}
		</Button>
		<Button
			variant="secondary"
			disabled={store.cdromInFlight || store.bootCdromInFlight || !isoMounted}
			onclick={requestBoot}
			data-testid="vm-cdrom-boot"
			title={isoMounted ? m['vms.disks.bootFromCdrom']() : m['vms.disks.bootFromCdromNeedsIso']()}
		>
			{#if store.bootCdromInFlight}
				<SpinnerIcon class="h-4 w-4" />
				{m['vms.disks.bootCdromBooting']()}
			{:else}
				<DiscIcon class="h-4 w-4" />
				{m['vms.disks.bootFromCdrom']()}
			{/if}
		</Button>
		<Button
			variant="secondary"
			disabled={store.cdromInFlight || occupied}
			onclick={() => store.setCdrom('disconnect')}
			data-testid="vm-cdrom-disconnect"
		>
			{m['vms.disks.disconnect']()}
		</Button>
		<Button
			variant="outline"
			class="border-destructive/30 text-destructive hover:border-destructive/50 hover:bg-destructive/10 hover:text-destructive"
			disabled={store.cdromInFlight || occupied}
			onclick={() => store.setCdrom('remove')}
			data-testid="vm-cdrom-remove"
		>
			{m['vms.disks.removeDrive']()}
		</Button>
	</div>
	{#if store.bootCdromError}
		<Alert data-testid="vm-cdrom-boot-error" class="mt-3">{store.bootCdromError}</Alert>
	{/if}
	{#if store.writeError}
		<Alert class="mt-3">{store.writeError}</Alert>
	{/if}
</Card>

<MountIsoDialog bind:open={mountOpen} />

<Dialog bind:open={bootConfirmOpen} labelledBy="boot-cdrom-title" onClose={() => (bootConfirmOpen = false)}>
	<h2 id="boot-cdrom-title" class="mb-2 text-lg font-semibold">{m['vms.disks.bootCdromConfirmTitle']()}</h2>
	<p class="text-sm text-muted-foreground">{m['vms.disks.bootCdromConfirmBody']()}</p>
	<div class="mt-4 flex justify-end gap-2">
		<Button variant="secondary" onclick={() => (bootConfirmOpen = false)}>{m['common.cancel']()}</Button>
		<Button
			loading={store.bootCdromInFlight}
			onclick={() => {
				bootConfirmOpen = false;
				void bootFromCdrom();
			}}
			data-testid="vm-cdrom-boot-confirm"
		>
			{m['vms.disks.bootFromCdrom']()}
		</Button>
	</div>
</Dialog>
