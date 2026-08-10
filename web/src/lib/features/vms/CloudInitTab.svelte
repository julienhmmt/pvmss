<script lang="ts">
	import { onMount } from 'svelte';
	import { getVmDetailContext } from './detail.svelte';
	import type { CloudInitConfigUpdate } from './cloudinit.svelte';
	import { CloudInitStore } from './cloudinit.svelte';
	import CloudInitForm from './CloudInitForm.svelte';
	import CloudInitSnippetEditor from './CloudInitSnippetEditor.svelte';
	import SaveCloudInitDialog from './SaveCloudInitDialog.svelte';

	const vmStore = getVmDetailContext();
	const cloudInit = new CloudInitStore(vmStore.cluster, vmStore.vmid, () => vmStore.load());
	let mode = $state<'structured' | 'yaml'>('structured');
	let saveDialogOpen = $state(false);
	let pendingUpdate = $state<CloudInitConfigUpdate | null>(null);

	onMount(() => {
		void cloudInit.loadConfig();
		void cloudInit.loadSnippet();
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

<section aria-labelledby="cloudinit-heading" data-testid="vm-cloudinit">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h2 id="cloudinit-heading" class="text-lg font-semibold">Cloud-init</h2>
			<p class="mt-1 text-sm text-muted-foreground">Configure live VM cloud-init settings or custom YAML.</p>
		</div>
		<div class="flex gap-2" role="group" aria-label="Cloud-init editor mode">
			<button
				type="button"
				class="rounded-md border px-3 py-2 text-sm {mode === 'structured' ? 'border-primary text-foreground' : 'border-border text-muted-foreground'}"
				aria-pressed={mode === 'structured'}
				onclick={() => (mode = 'structured')}
				data-testid="cloudinit-mode-structured"
			>
				Structured
			</button>
			<button
				type="button"
				class="rounded-md border px-3 py-2 text-sm {mode === 'yaml' ? 'border-primary text-foreground' : 'border-border text-muted-foreground'}"
				aria-pressed={mode === 'yaml'}
				onclick={() => (mode = 'yaml')}
				data-testid="cloudinit-mode-yaml"
			>
				YAML editor
			</button>
		</div>
	</div>

	<div class="mt-6">
		{#if mode === 'structured'}
			<CloudInitForm store={cloudInit} onRequestSave={requestSave} />
		{:else}
			<CloudInitSnippetEditor store={cloudInit} />
		{/if}
	</div>

	<SaveCloudInitDialog bind:open={saveDialogOpen} saving={cloudInit.configInFlight} onConfirm={confirmSave} onClose={closeSaveDialog} />
</section>
