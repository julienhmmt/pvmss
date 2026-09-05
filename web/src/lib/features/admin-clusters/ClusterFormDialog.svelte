<script lang="ts">
	import { untrack } from 'svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Checkbox from '$lib/shared/ui/Checkbox.svelte';
	import type { AdminCluster, ClusterInput } from './clusters.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		open: boolean;
		editing: AdminCluster | null;
		saving: boolean;
		error: string | null;
		onClose: () => void;
		onSubmit: (input: ClusterInput) => void;
	}

	let { open = $bindable(false), editing, saving, error, onClose, onSubmit }: Props = $props();
	let name = $state('');
	let url = $state('');
	let tokenId = $state('');
	let tokenSecret = $state('');
	let tlsInsecureSkipVerify = $state(false);
	const TITLE_ID = 'cluster-form-title';

	$effect(() => {
		if (!open) return;
		untrack(() => {
			name = editing?.name ?? '';
			url = editing?.url ?? '';
			tokenId = editing?.tokenId ?? '';
			tokenSecret = '';
			tlsInsecureSkipVerify = editing?.tlsInsecureSkipVerify ?? false;
		});
	});

	function submit(): void {
		onSubmit({ name: name.trim(), url: url.trim(), tokenId: tokenId.trim(), tokenSecret, tlsInsecureSkipVerify });
	}
</script>

<Dialog bind:open labelledBy={TITLE_ID} onClose={onClose}>
	<h2 id={TITLE_ID} class="text-lg font-semibold">{editing ? m['admin.clusters.editCluster']() : m['admin.clusters.addClusterForm']()}</h2>
	<form class="mt-4 grid gap-4" onsubmit={(event) => { event.preventDefault(); submit(); }}>
		<FormField label={m['common.name']()} required>
			{#snippet children({ id, describedBy, invalid })}
				<TextField {id} {describedBy} {invalid} bind:value={name} disabled={editing !== null} pattern="[a-z0-9-]+" required />
			{/snippet}
		</FormField>
		<FormField label={m['admin.clusters.url']()} required>
			{#snippet children({ id, describedBy, invalid })}
				<TextField {id} {describedBy} {invalid} type="url" bind:value={url} required />
			{/snippet}
		</FormField>
		<FormField label={m['admin.clusters.tokenId']()} required>
			{#snippet children({ id, describedBy, invalid })}
				<TextField {id} {describedBy} {invalid} bind:value={tokenId} required />
			{/snippet}
		</FormField>
		<FormField label={m['admin.clusters.tokenSecret']()} hint={editing ? m['admin.clusters.tokenSecretHint']() : undefined} required={editing === null}>
			{#snippet children({ id, describedBy, invalid })}
				<TextField {id} {describedBy} {invalid} type="password" bind:value={tokenSecret} reveal required={editing === null} autocomplete="new-password" />
			{/snippet}
		</FormField>
		<Checkbox
			label={m['admin.clusters.skipTls']()}
			checked={tlsInsecureSkipVerify}
			onToggle={(checked) => (tlsInsecureSkipVerify = checked)}
			variant="warning"
		/>
		{#if error}
			<Alert>{error}</Alert>
		{/if}
		<div class="mt-2 flex justify-end gap-2">
			<Button variant="secondary" onclick={onClose} disabled={saving}>{m['common.cancel']()}</Button>
			<Button type="submit" disabled={saving}>{m['common.save']()}</Button>
		</div>
	</form>
</Dialog>
