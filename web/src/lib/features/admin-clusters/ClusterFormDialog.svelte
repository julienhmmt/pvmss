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
	import Pill from '$lib/shared/ui/Pill.svelte';
	import { publishingOffHint } from './publishing-status';
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
			{#if editing && !editing.cloudInitWriteEnabled}
				<Alert tone="info" role="status">{publishingOffHint(editing.publishingStatus)}</Alert>
			{/if}

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

			<div class="grid gap-1.5">
				<div class="grid gap-4 sm:grid-cols-[minmax(0,1fr)_7rem]">
					<FormField label={m['admin.clusters.sshUser']()}>
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
				<p class="text-xs text-muted-foreground">{m['admin.clusters.sshUserHint']()}</p>
			</div>

			<div class="grid gap-2">
				<FormField label={m['admin.clusters.sshKnownHosts']()} hint={m['admin.clusters.sshKnownHostsHint']()}>
					{#snippet children({ id, describedBy, invalid })}
						<Textarea {id} {describedBy} {invalid} bind:value={sshKnownHosts} rows={4} mono placeholder="10.0.0.11 ssh-ed25519 AAAA..." />
					{/snippet}
				</FormField>
				<div class="flex flex-wrap items-center justify-between gap-2">
					<span class="text-xs text-muted-foreground">
						{#if editing === null}{m['admin.clusters.sshScanAfterSave']()}{/if}
					</span>
					<Button variant="secondary" size="sm" loading={scanning} disabled={editing === null} onclick={() => void scan()} data-testid="cluster-ssh-scan">
						{m['admin.clusters.sshScan']()}
					</Button>
				</div>
				{#if scanErrors.length > 0}
					<Alert tone="warning" role="status">
						<ul class="grid gap-1">
							{#each scanErrors as scanError (scanError)}
								<li class="break-words">{scanError}</li>
							{/each}
						</ul>
					</Alert>
				{/if}
			</div>

			<div class="grid min-w-0 gap-2 border-t border-border pt-4">
				{#if sshPublicKey}
					<div>
						<p class="text-sm font-medium">{m['admin.clusters.sshNodeSetup']()}</p>
						<p class="mt-0.5 text-xs text-muted-foreground">{m['admin.clusters.sshNodeSetupHint']()}</p>
					</div>
					<div class="flex min-w-0 items-start gap-2">
						<pre class="min-w-0 flex-1 whitespace-pre-wrap break-all rounded-md border border-border bg-background px-3 py-2 font-mono text-xs leading-relaxed" data-testid="cluster-ssh-setup">{setupCommand}</pre>
						<CopyButton value={setupCommand} />
					</div>
				{:else}
					<Alert tone="warning" role="status" data-testid="cluster-ssh-no-key">{m['admin.clusters.sshNoKey']()}</Alert>
				{/if}
			</div>
		</FormSection>

		{#if pairError ?? error}
			<Alert>{pairError ?? error}</Alert>
		{/if}
	</form>
	{#snippet footer()}
		<Button variant="ghost" onclick={onClose} disabled={saving}>{m['common.cancel']()}</Button>
		<Button type="submit" form={FORM_ID} loading={saving} disabled={saving}>{m['common.save']()}</Button>
	{/snippet}
</Dialog>
