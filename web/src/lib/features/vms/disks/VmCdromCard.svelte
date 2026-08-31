<script lang="ts">
	import { getVmDetailContext } from '../detail.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import MountIsoDialog from './MountIsoDialog.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import DiscIcon from '$lib/shared/ui/icons/DiscIcon.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getVmDetailContext();
	const toast = getToastContext();
	let mountOpen = $state(false);
	let bootConfirmOpen = $state(false);

	const isoMounted = $derived(store.entity?.cdrom?.state === 'mounted');

	async function bootFromCdrom(): Promise<void> {
		const vmName = store.entity?.name ?? '';
		const success = await store.bootFromCdrom();
		if (success) {
			toast.success(m['toast.vmBootedFromCdrom']({ name: vmName }));
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

<div class="rounded-xl border border-border bg-card p-6 shadow-card" data-testid="vm-cdrom">
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
	<div class="mt-5 flex flex-wrap gap-2">
		<button
			type="button"
			class="inline-flex items-center gap-1.5 rounded-lg bg-primary px-3 py-2 text-sm font-semibold text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
			disabled={store.cdromInFlight}
			onclick={() => (mountOpen = true)}
			data-testid="vm-cdrom-mount-open"
		>
			{m['vms.disks.mount']()}
		</button>
		<button
			type="button"
			class="inline-flex items-center gap-1.5 rounded-lg border border-border px-3 py-2 text-sm font-medium hover:bg-muted disabled:opacity-50"
			disabled={store.cdromInFlight || store.bootCdromInFlight || !isoMounted}
			onclick={requestBoot}
			data-testid="vm-cdrom-boot"
			title={isoMounted ? m['vms.disks.bootFromCdrom']() : m['vms.disks.bootFromCdromNeedsIso']()}
		>
			<DiscIcon class="h-4 w-4" />
			{m['vms.disks.bootFromCdrom']()}
		</button>
		<button
			type="button"
			class="rounded-lg border border-border px-3 py-2 text-sm font-medium hover:bg-muted disabled:opacity-50"
			disabled={store.cdromInFlight}
			onclick={() => store.setCdrom('disconnect')}
			data-testid="vm-cdrom-disconnect"
		>
			{m['vms.disks.disconnect']()}
		</button>
		<button
			type="button"
			class="rounded-lg border border-destructive/30 px-3 py-2 text-sm font-medium text-destructive hover:bg-destructive/10 disabled:opacity-50"
			disabled={store.cdromInFlight}
			onclick={() => store.setCdrom('remove')}
			data-testid="vm-cdrom-remove"
		>
			{m['vms.disks.removeDrive']()}
		</button>
	</div>
	{#if store.bootCdromError}
		<p class="mt-3 text-sm text-destructive" role="alert" data-testid="vm-cdrom-boot-error">{store.bootCdromError}</p>
	{/if}
	{#if store.writeError}
		<p class="mt-3 text-sm text-destructive" role="alert">{store.writeError}</p>
	{/if}
</div>

<MountIsoDialog bind:open={mountOpen} />

<Dialog bind:open={bootConfirmOpen} labelledBy="boot-cdrom-title" onClose={() => (bootConfirmOpen = false)}>
	<h2 id="boot-cdrom-title" class="mb-2 text-lg font-semibold">{m['vms.disks.bootCdromConfirmTitle']()}</h2>
	<p class="text-sm text-muted-foreground">{m['vms.disks.bootCdromConfirmBody']()}</p>
	<div class="mt-4 flex justify-end gap-2">
		<button
			type="button"
			class="rounded-md border border-border px-4 py-2 text-sm hover:bg-muted"
			onclick={() => (bootConfirmOpen = false)}
		>
			{m['common.cancel']()}
		</button>
		<button
			type="button"
			class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
			disabled={store.bootCdromInFlight}
			onclick={() => {
				bootConfirmOpen = false;
				void bootFromCdrom();
			}}
			data-testid="vm-cdrom-boot-confirm"
		>
			{m['vms.disks.bootFromCdrom']()}
		</button>
	</div>
</Dialog>
