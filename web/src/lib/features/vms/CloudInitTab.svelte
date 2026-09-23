<script lang="ts">
	import { onMount } from 'svelte';
	import { getVmDetailContext } from './detail.svelte';
	import type { CloudInitConfigUpdate } from './cloudinit.svelte';
	import { CloudInitStore } from './cloudinit.svelte';
	import CloudInitForm from './CloudInitForm.svelte';
	import CloudInitDocumentPicker from './CloudInitDocumentPicker.svelte';
	import SaveCloudInitDialog from './SaveCloudInitDialog.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const vmStore = getVmDetailContext();
	const cloudInit = new CloudInitStore(vmStore.cluster, vmStore.vmid, () => vmStore.load());
	let mode = $state<'structured' | 'document'>('structured');
	let saveDialogOpen = $state(false);
	let pendingUpdate = $state<CloudInitConfigUpdate | null>(null);

	onMount(() => {
		void cloudInit.loadConfig();
	});

	function requestSave(update: CloudInitConfigUpdate): void {
		pendingUpdate = update;
		saveDialogOpen = true;
	}

	async function confirmSave(rebootNow: boolean): Promise<void> {
		if (pendingUpdate === null) return;
		const saved = await cloudInit.saveConfig(pendingUpdate, rebootNow);
		if (!saved) return;
		pendingUpdate = null;
		saveDialogOpen = false;
	}

	function closeSaveDialog(): void {
		saveDialogOpen = false;
		pendingUpdate = null;
	}
</script>

<section class="rounded-xl border border-border bg-card p-6 shadow-card" aria-labelledby="cloudinit-heading" data-testid="vm-cloudinit">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h2 id="cloudinit-heading" class="text-lg font-semibold">{m['vms.cloudinit.heading']()}</h2>
			<p class="mt-1 text-sm text-muted-foreground">{m['vms.cloudinit.description']()}</p>
		</div>
		<div class="inline-flex rounded-lg border border-border bg-muted/40 p-0.5" role="group" aria-label={m['vms.cloudinit.modeLabel']()}>
			<button
				type="button"
				class="rounded-md px-3 py-1.5 text-sm font-medium {mode === 'structured'
					? 'bg-card text-foreground shadow-card'
					: 'text-muted-foreground hover:text-foreground'}"
				aria-pressed={mode === 'structured'}
				onclick={() => (mode = 'structured')}
				data-testid="cloudinit-mode-structured"
			>
				{m['vms.cloudinit.modeStructured']()}
			</button>
			<button
				type="button"
				class="rounded-md px-3 py-1.5 text-sm font-medium {mode === 'document'
					? 'bg-card text-foreground shadow-card'
					: 'text-muted-foreground hover:text-foreground'}"
				aria-pressed={mode === 'document'}
				onclick={() => (mode = 'document')}
				data-testid="cloudinit-mode-document"
			>
				{m['vms.cloudinit.modeDocument']()}
			</button>
		</div>
	</div>

	<div class="mt-6">
		{#if mode === 'structured'}
			<CloudInitForm store={cloudInit} onRequestSave={requestSave} />
		{:else}
			<CloudInitDocumentPicker store={cloudInit} />
		{/if}
	</div>

	<SaveCloudInitDialog bind:open={saveDialogOpen} saving={cloudInit.configInFlight} onConfirm={confirmSave} onClose={closeSaveDialog} />
</section>
