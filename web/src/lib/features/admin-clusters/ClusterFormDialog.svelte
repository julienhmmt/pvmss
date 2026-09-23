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
	import Textarea from '$lib/shared/ui/Textarea.svelte';
	import CopyButton from '$lib/shared/ui/CopyButton.svelte';
	import type { AdminCluster, ClusterInput, HostKeyScan, SnippetStorage } from './clusters.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		open: boolean;
		editing: AdminCluster | null;
		saving: boolean;
		error: string | null;
		snippetStorages: SnippetStorage[];
		snippetStoragesLoading: boolean;
		/** PVMSS's public key, shown for the node setup ('' = no key file). */
		sshPublicKey: string;
		onClose: () => void;
		onSubmit: (input: ClusterInput) => void;
		/** Scans the nodes' host keys of an existing cluster. */
		onScan: (cluster: string) => Promise<HostKeyScan[]>;
	}

	let {
		open = $bindable(false),
		editing,
		saving,
		error,
		snippetStorages,
		snippetStoragesLoading,
		sshPublicKey,
		onClose,
		onSubmit,
		onScan
	}: Props = $props();
	let name = $state('');
	let url = $state('');
	let tokenId = $state('');
	let tokenSecret = $state('');
	let tlsInsecureSkipVerify = $state(false);
	let snippetStorage = $state('');
	let sshUser = $state('');
	let sshPort = $state('22');
	let sshKnownHosts = $state('');
	let scanning = $state(false);
	let scanErrors = $state<string[]>([]);
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
			snippetStorage = editing?.snippetStorage ?? '';
			sshUser = editing?.sshUser ?? '';
			sshPort = String(editing?.sshPort || 22);
			sshKnownHosts = editing?.sshKnownHosts ?? '';
			scanErrors = [];
			pairError = null;
		});
	});

	// The command an admin runs on every node, prefilled with the storage
	// and PVMSS's key (tools/pvmss-node-setup.sh).
	const setupCommand = $derived(
		`sh pvmss-node-setup.sh --storage ${snippetStorage.trim() || 'local'} --user ${sshUser.trim() || 'pvmss'} --key '${sshPublicKey}'`
	);

	async function scan(): Promise<void> {
		if (!editing) return;
		scanning = true;
		scanErrors = [];
		try {
			const scans = await onScan(editing.name);
			const lines = scans.filter((s) => s.line).map((s) => s.line as string);
			scanErrors = scans.filter((s) => s.error).map((s) => `${s.node}: ${s.error}`);
			if (lines.length > 0) sshKnownHosts = lines.join('\n');
		} catch (err) {
			scanErrors = [err instanceof Error ? err.message : String(err)];
		} finally {
			scanning = false;
		}
	}

	function submit(): void {
		const storage = snippetStorage.trim();
		const user = sshUser.trim();
		const port = Number.parseInt(sshPort, 10);
		if (user !== '' && sshKnownHosts.trim() === '') {
			pairError = m['admin.clusters.sshKnownHostsRequired']();
			return;
		}
		if (!Number.isInteger(port) || port < 1 || port > 65535) {
			pairError = m['admin.clusters.sshPortInvalid']();
			return;
		}
		pairError = null;
		onSubmit({
			name: name.trim(),
			url: url.trim(),
			tokenId: tokenId.trim(),
			tokenSecret,
			tlsInsecureSkipVerify,
			snippetStorage: storage,
			sshUser: user,
			sshPort: port,
			sshKnownHosts: sshKnownHosts.trim()
		});
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
			<div class="grid gap-4 sm:grid-cols-[1fr_8rem]">
				<FormField label={m['admin.clusters.sshUser']()} hint={m['admin.clusters.sshUserHint']()}>
					{#snippet children({ id, describedBy, invalid })}
						<TextField {id} {describedBy} {invalid} bind:value={sshUser} placeholder="pvmss" autocomplete="off" />
					{/snippet}
				</FormField>
				<FormField label={m['admin.clusters.sshPort']()}>
					{#snippet children({ id, describedBy, invalid })}
						<TextField {id} {describedBy} {invalid} type="number" bind:value={sshPort} />
					{/snippet}
				</FormField>
			</div>
			<FormField label={m['admin.clusters.sshKnownHosts']()} hint={m['admin.clusters.sshKnownHostsHint']()}>
				{#snippet children({ id, describedBy, invalid })}
					<Textarea {id} {describedBy} {invalid} bind:value={sshKnownHosts} rows={3} mono placeholder="10.0.0.11 ssh-ed25519 AAAA..." />
				{/snippet}
			</FormField>
			<div class="flex flex-wrap items-center gap-2">
				<Button variant="secondary" size="sm" loading={scanning} disabled={editing === null} onclick={() => void scan()} data-testid="cluster-ssh-scan">
					{m['admin.clusters.sshScan']()}
				</Button>
				{#if editing === null}
					<span class="text-xs text-muted-foreground">{m['admin.clusters.sshScanAfterSave']()}</span>
				{/if}
			</div>
			{#each scanErrors as scanError (scanError)}
				<p class="text-xs text-warning">{scanError}</p>
			{/each}
			{#if sshPublicKey}
				<div class="grid gap-1.5 text-sm">
					<span class="font-medium">{m['admin.clusters.sshNodeSetup']()}</span>
					<div class="flex items-start gap-2">
						<code class="block flex-1 overflow-x-auto rounded-md bg-muted px-2 py-1.5 font-mono text-xs" data-testid="cluster-ssh-setup">{setupCommand}</code>
						<CopyButton value={setupCommand} />
					</div>
				</div>
			{:else}
				<p class="text-xs text-warning" data-testid="cluster-ssh-no-key">{m['admin.clusters.sshNoKey']()}</p>
			{/if}
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
