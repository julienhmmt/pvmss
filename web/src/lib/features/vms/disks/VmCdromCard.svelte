<script lang="ts">
	import { getVmDetailContext } from '../detail.svelte';
	import MountIsoDialog from './MountIsoDialog.svelte';

	const store = getVmDetailContext();
	let mountOpen = $state(false);
</script>

<div class="rounded-md border border-border p-4" data-testid="vm-cdrom">
	<h2 class="text-lg font-semibold">CD-ROM</h2>
	<p class="mt-1 text-sm text-muted-foreground" data-testid="vm-cdrom-state">
		State: {store.entity?.cdrom?.state ?? 'absent'}
	</p>
	{#if store.entity?.cdrom?.isoVolId}
		<p class="mt-1 truncate text-xs text-muted-foreground">{store.entity.cdrom.isoVolId}</p>
	{/if}
	<div class="mt-4 flex flex-wrap gap-2">
		<button
			type="button"
			class="rounded-md bg-primary px-3 py-2 text-sm text-primary-foreground disabled:opacity-50"
			disabled={store.cdromInFlight}
			onclick={() => (mountOpen = true)}
			data-testid="vm-cdrom-mount-open"
		>
			Mount ISO
		</button>
		<button
			type="button"
			class="rounded-md border border-border px-3 py-2 text-sm disabled:opacity-50"
			disabled={store.cdromInFlight}
			onclick={() => store.setCdrom('disconnect')}
			data-testid="vm-cdrom-disconnect"
		>
			Disconnect
		</button>
		<button
			type="button"
			class="rounded-md border border-destructive/30 px-3 py-2 text-sm text-destructive disabled:opacity-50"
			disabled={store.cdromInFlight}
			onclick={() => store.setCdrom('remove')}
			data-testid="vm-cdrom-remove"
		>
			Remove drive
		</button>
	</div>
	{#if store.writeError}
		<p class="mt-2 text-sm text-destructive" role="alert">{store.writeError}</p>
	{/if}
</div>

<MountIsoDialog bind:open={mountOpen} />
