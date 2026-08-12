<script lang="ts">
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import type { AdminCluster, ClusterInput } from './clusters.svelte';

	interface Props {
		open: boolean;
		editing: AdminCluster | null;
		onClose: () => void;
		onSubmit: (input: ClusterInput) => void;
	}

	let { open = $bindable(false), editing, onClose, onSubmit }: Props = $props();
	let name = $state('');
	let url = $state('');
	let tokenId = $state('');
	let tokenSecret = $state('');
	let tlsInsecureSkipVerify = $state(false);
	const TITLE_ID = 'cluster-form-title';

	$effect(() => {
		if (!open) return;
		name = editing?.name ?? '';
		url = editing?.url ?? '';
		tokenId = editing?.tokenId ?? '';
		tokenSecret = '';
		tlsInsecureSkipVerify = editing?.tlsInsecureSkipVerify ?? false;
	});

	function submit(): void {
		onSubmit({ name: name.trim(), url: url.trim(), tokenId: tokenId.trim(), tokenSecret, tlsInsecureSkipVerify });
	}
</script>

<Dialog bind:open labelledBy={TITLE_ID} onClose={onClose}>
	<h2 id={TITLE_ID} class="text-lg font-semibold">{editing ? 'Edit cluster' : 'Add cluster'}</h2>
	<form class="mt-4 grid gap-3" onsubmit={(event) => { event.preventDefault(); submit(); }}>
		<label class="grid gap-1 text-sm font-medium">
			Name
			<input class="rounded-md border border-input bg-background px-3 py-2" bind:value={name} disabled={editing !== null} pattern="[a-z0-9-]+" required />
		</label>
		<label class="grid gap-1 text-sm font-medium">
			URL
			<input class="rounded-md border border-input bg-background px-3 py-2" type="url" bind:value={url} required />
		</label>
		<label class="grid gap-1 text-sm font-medium">
			Token ID
			<input class="rounded-md border border-input bg-background px-3 py-2" bind:value={tokenId} required />
		</label>
		<label class="grid gap-1 text-sm font-medium">
			Token secret {#if editing}<span class="font-normal text-muted-foreground">(leave blank to keep it)</span>{/if}
			<input class="rounded-md border border-input bg-background px-3 py-2" type="password" bind:value={tokenSecret} required={editing === null} autocomplete="new-password" />
		</label>
		<label class="flex items-center gap-2 text-sm">
			<input type="checkbox" bind:checked={tlsInsecureSkipVerify} /> Skip TLS certificate verification
		</label>
		<div class="mt-2 flex justify-end gap-2">
			<button type="button" class="rounded-md border border-border px-3 py-2 text-sm" onclick={onClose}>Cancel</button>
			<button type="submit" class="rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground">Save</button>
		</div>
	</form>
</Dialog>
