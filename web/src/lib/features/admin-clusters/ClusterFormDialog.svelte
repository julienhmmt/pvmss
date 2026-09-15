<script lang="ts">
	import { untrack } from 'svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import FormSection from '$lib/shared/ui/FormSection.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import Checkbox from '$lib/shared/ui/Checkbox.svelte';
	import type { AdminCluster, ClusterInput, SnippetStorage } from './clusters.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		open: boolean;
		editing: AdminCluster | null;
		saving: boolean;
		error: string | null;
		snippetStorages: SnippetStorage[];
		snippetStoragesLoading: boolean;
		onClose: () => void;
		onSubmit: (input: ClusterInput) => void;
	}

	let {
		open = $bindable(false),
		editing,
		saving,
		error,
		snippetStorages,
		snippetStoragesLoading,
		onClose,
		onSubmit
	}: Props = $props();
	let name = $state('');
	let url = $state('');
	let tokenId = $state('');
	let tokenSecret = $state('');
	let tlsInsecureSkipVerify = $state(false);
	let snippetDir = $state('');
	let snippetStorage = $state('');
	let pairError = $state<string | null>(null);
	const TITLE_ID = 'cluster-form-title';

	$effect(() => {
		if (!open) return;
		untrack(() => {
			name = editing?.name ?? '';
			url = editing?.url ?? '';
			tokenId = editing?.tokenId ?? '';
			tokenSecret = '';
			tlsInsecureSkipVerify = editing?.tlsInsecureSkipVerify ?? false;
			snippetDir = editing?.snippetDir ?? '';
			snippetStorage = editing?.snippetStorage ?? '';
			pairError = null;
		});
	});

	// Auto-fill the snippet directory with the standard mount path when the
	// admin picks a storage and hasn't typed a custom path. The Helm chart
	// and docker-compose examples both mount at /snippets.
	$effect(() => {
		if (snippetStorage && !snippetDir) {
			snippetDir = '/snippets';
		}
	});

	function submit(): void {
		const dir = snippetDir.trim();
		const storage = snippetStorage.trim();
		if ((dir === '') !== (storage === '')) {
			pairError = m['admin.clusters.cloudinitPairError']();
			return;
		}
		pairError = null;
		onSubmit({ name: name.trim(), url: url.trim(), tokenId: tokenId.trim(), tokenSecret, tlsInsecureSkipVerify, snippetDir: dir, snippetStorage: storage });
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
		<FormSection legend={m['admin.clusters.cloudinitSection']()} description={m['admin.clusters.cloudinitHint']()}>
			<FormField label={m['admin.clusters.snippetDir']()} hint={m['admin.clusters.snippetDirHint']()}>
				{#snippet children({ id, describedBy, invalid })}
					<TextField {id} {describedBy} {invalid} bind:value={snippetDir} placeholder="/snippets" />
				{/snippet}
			</FormField>
			<FormField
				label={m['admin.clusters.snippetStorage']()}
				hint={snippetStoragesLoading ? m['admin.clusters.snippetStorageLoading']() : (snippetStorages.length > 0 ? m['admin.clusters.snippetStorageHint']() : m['admin.clusters.snippetStorageEmpty']())}
			>
				{#snippet children({ id, describedBy, invalid })}
					{#if snippetStorages.length > 0}
						<Select
							{id}
							{describedBy}
							{invalid}
							bind:value={snippetStorage}
							placeholder={m['admin.clusters.snippetStoragePlaceholder']()}
							options={snippetStorages.map((s) => ({ value: s.name, label: `${s.name} (${s.node})` }))}
						/>
					{:else}
						<TextField {id} {describedBy} {invalid} bind:value={snippetStorage} placeholder="shared" disabled={snippetStoragesLoading} />
					{/if}
				{/snippet}
			</FormField>
		</FormSection>
		{#if pairError ?? error}
			<Alert>{pairError ?? error}</Alert>
		{/if}
		<div class="mt-2 flex justify-end gap-2">
			<Button variant="secondary" onclick={onClose} disabled={saving}>{m['common.cancel']()}</Button>
			<Button type="submit" disabled={saving}>{m['common.save']()}</Button>
		</div>
	</form>
</Dialog>
