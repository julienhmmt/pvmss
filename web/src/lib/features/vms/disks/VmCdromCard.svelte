<script lang="ts">
	import { getVmDetailContext } from '../detail.svelte';
	import MountIsoDialog from './MountIsoDialog.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getVmDetailContext();
	let mountOpen = $state(false);
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
			class="rounded-lg bg-primary px-3 py-2 text-sm font-semibold text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
			disabled={store.cdromInFlight}
			onclick={() => (mountOpen = true)}
			data-testid="vm-cdrom-mount-open"
		>
			{m['vms.disks.mount']()}
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
	{#if store.writeError}
		<p class="mt-3 text-sm text-destructive" role="alert">{store.writeError}</p>
	{/if}
</div>

<MountIsoDialog bind:open={mountOpen} />
