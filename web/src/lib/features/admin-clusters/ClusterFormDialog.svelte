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
	import Pill from '$lib/shared/ui/Pill.svelte';
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
	let snippetStorage = $state('');
	const TITLE_ID = 'cluster-form-title';
	const FORM_ID = 'cluster-form';

	$effect(() => {
		if (!open) return;
		untrack(() => {
			name = editing?.name ?? '';
			url = editing?.url ?? '';
			tokenId = editing?.tokenId ?? '';
			tokenSecret = '';
			tlsInsecureSkipVerify = editing?.tlsInsecureSkipVerify ?? false;
			snippetStorage = editing?.snippetStorage ?? '';
		});
	});

	function submit(): void {
		onSubmit({
			name: name.trim(),
			url: url.trim(),
			tokenId: tokenId.trim(),
			tokenSecret,
			tlsInsecureSkipVerify,
			snippetStorage: snippetStorage.trim()
		});
	}
</script>

<Dialog bind:open size="xl" labelledBy={TITLE_ID} onClose={onClose}>
	<h2 id={TITLE_ID} class="text-lg font-semibold">{editing ? m['admin.clusters.editCluster']() : m['admin.clusters.addClusterForm']()}</h2>
	<form id={FORM_ID} class="mt-5 grid gap-6" onsubmit={(event) => { event.preventDefault(); submit(); }}>
		<FormSection legend={m['admin.clusters.connectionSection']()} description={m['admin.clusters.connectionHint']()}>
			<div class="grid gap-4 sm:grid-cols-2">
				<FormField label={m['common.name']()} required>
					{#snippet children({ id, describedBy, invalid })}
						<TextField {id} {describedBy} {invalid} bind:value={name} disabled={editing !== null} pattern="[a-z0-9-]+" required />
					{/snippet}
				</FormField>
				<FormField label={m['admin.clusters.url']()} required>
					{#snippet children({ id, describedBy, invalid })}
						<TextField {id} {describedBy} {invalid} type="url" bind:value={url} placeholder="https://pve.example:8006/api2/json" required />
					{/snippet}
				</FormField>
				<FormField label={m['admin.clusters.tokenId']()} required>
					{#snippet children({ id, describedBy, invalid })}
						<TextField {id} {describedBy} {invalid} bind:value={tokenId} placeholder="pvmss@pve!service" required />
					{/snippet}
				</FormField>
				<FormField label={m['admin.clusters.tokenSecret']()} required={editing === null}>
					{#snippet children({ id, describedBy, invalid })}
						<TextField
							{id}
							{describedBy}
							{invalid}
							type="password"
							bind:value={tokenSecret}
							reveal
							required={editing === null}
							autocomplete="new-password"
							placeholder={editing ? m['admin.clusters.tokenSecretHint']() : ''}
						/>
					{/snippet}
				</FormField>
			</div>
			<Checkbox
				label={m['admin.clusters.skipTls']()}
				checked={tlsInsecureSkipVerify}
				onToggle={(checked) => (tlsInsecureSkipVerify = checked)}
				variant="warning"
			/>
		</FormSection>

		<FormSection legend={m['admin.clusters.cloudinitSection']()} description={m['admin.clusters.cloudinitHint']()} variant="panel">
			{#snippet actions()}
				{#if editing}
					<Pill
						tone={editing.cloudInitWriteEnabled ? 'ok' : 'off'}
						label={editing.cloudInitWriteEnabled ? m['admin.clusters.cloudinitOn']() : m['admin.clusters.cloudinitOff']()}
					/>
				{/if}
			{/snippet}
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
						<TextField {id} {describedBy} {invalid} bind:value={snippetStorage} placeholder="local" disabled={snippetStoragesLoading} />
					{/if}
				{/snippet}
			</FormField>

		</FormSection>

		{#if error}
			<Alert>{error}</Alert>
		{/if}
	</form>
	{#snippet footer()}
		<Button variant="ghost" onclick={onClose} disabled={saving}>{m['common.cancel']()}</Button>
		<Button type="submit" form={FORM_ID} loading={saving} disabled={saving}>{m['common.save']()}</Button>
	{/snippet}
</Dialog>
