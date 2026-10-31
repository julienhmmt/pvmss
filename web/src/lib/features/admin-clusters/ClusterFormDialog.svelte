<script lang="ts">
	import { untrack } from 'svelte';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Dialog from '#lib/shared/ui/Dialog.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import FormField from '#lib/shared/ui/FormField.svelte';
	import FormSection from '#lib/shared/ui/FormSection.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';
	import Checkbox from '#lib/shared/ui/Checkbox.svelte';
	import Textarea from '#lib/shared/ui/Textarea.svelte';
	import type { AdminCluster, ClusterInput, ClusterTlsInput } from './clusters.svelte';
	import { formatDate } from '#lib/shared/format-date.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		open: boolean;
		editing: AdminCluster | null;
		saving: boolean;
		error: string | null;
		onClose: () => void;
		onSubmit: (input: ClusterInput) => void;
	}

	let {
		open = $bindable(false),
		editing,
		saving,
		error,
		onClose,
		onSubmit
	}: Props = $props();
	let name = $state('');
	let displayName = $state('');
	let url = $state('');
	let tokenId = $state('');
	let tokenSecret = $state('');
	let tlsInsecureSkipVerify = $state(false);
	let caPem = $state('');
	let nodeFingerprints = $state('');
	let clientCertPem = $state('');
	let clientKeyPem = $state('');
	let clearCa = $state(false);
	let clearClient = $state(false);
	const tlsInfo = $derived(editing?.tls);
	// Changing what the cluster is trusted against must re-enter the secret.
	const trustChanged = $derived(
		editing !== null &&
			(caPem.trim() !== '' ||
				clearCa ||
				nodeFingerprints.trim() !== (editing.tls?.nodeFingerprints ?? []).join('\n'))
	);
	// The server sends the kept secret to the cluster URL, so it refuses a new
	// URL or TLS setting without the secret re-entered.
	const secretRequired = $derived(
		editing === null || url.trim() !== editing.url || tlsInsecureSkipVerify !== editing.tlsInsecureSkipVerify || trustChanged
	);

	function showDate(value: string | null): string {
		return value ? formatDate(value) : '';
	}

	function tlsInput(): ClusterTlsInput {
		const tls: ClusterTlsInput = { nodeFingerprints: nodeFingerprints.trim() };
		if (clearCa) tls.caPem = '';
		else if (caPem.trim() !== '') tls.caPem = caPem;
		if (clearClient) {
			tls.clientCertPem = '';
			tls.clientKeyPem = '';
		} else if (clientCertPem.trim() !== '' || clientKeyPem.trim() !== '') {
			tls.clientCertPem = clientCertPem;
			tls.clientKeyPem = clientKeyPem;
		}
		return tls;
	}
	const TITLE_ID = 'cluster-form-title';
	const FORM_ID = 'cluster-form';

	$effect(() => {
		if (!open) return;
		untrack(() => {
			name = editing?.name ?? '';
			displayName = editing?.displayName ?? '';
			url = editing?.url ?? '';
			tokenId = editing?.tokenId ?? '';
			tokenSecret = '';
			tlsInsecureSkipVerify = editing?.tlsInsecureSkipVerify ?? false;

			caPem = '';
			clientCertPem = '';
			clientKeyPem = '';
			clearCa = false;
			clearClient = false;
			nodeFingerprints = (editing?.tls?.nodeFingerprints ?? []).join('\n');
		});
	});

	function submit(): void {
		onSubmit({
			name: name.trim(),
			displayName: displayName.trim(),
			url: url.trim(),
			tokenId: tokenId.trim(),
			tokenSecret,
			tlsInsecureSkipVerify,

			tls: tlsInput()
		});
	}
</script>

<Dialog bind:open size="xl" labelledBy={TITLE_ID} onClose={onClose}>
	<h2 id={TITLE_ID} class="text-lg font-semibold">{editing ? m['admin.clusters.editCluster']() : m['admin.clusters.addClusterForm']()}</h2>
	<form id={FORM_ID} class="mt-5 grid gap-6" onsubmit={(event) => { event.preventDefault(); submit(); }}>
		<FormSection legend={m['admin.clusters.connectionSection']()} description={m['admin.clusters.connectionHint']()}>
			<div class="grid gap-4 sm:grid-cols-2">
				<FormField label={m['common.name']()} required hint={m['admin.clusters.idHint']()}>
					{#snippet children({ id, describedBy, invalid })}
						<TextField {id} {describedBy} {invalid} bind:value={name} disabled={editing !== null} pattern="[a-z0-9-]+" required />
					{/snippet}
				</FormField>
				<FormField label={m['admin.clusters.displayName']()} hint={m['admin.clusters.displayNameHint']()}>
					{#snippet children({ id, describedBy, invalid })}
						<TextField {id} {describedBy} {invalid} bind:value={displayName} />
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
				<FormField label={m['admin.clusters.tokenSecret']()} required={secretRequired}>
					{#snippet children({ id, describedBy, invalid })}
						<TextField
							{id}
							{describedBy}
							{invalid}
							type="password"
							bind:value={tokenSecret}
							reveal
							required={secretRequired}
							autocomplete="new-password"
							placeholder={secretRequired ? '' : m['admin.clusters.tokenSecretHint']()}
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

		<FormSection legend={m['admin.clusters.tlsSection']()} description={m['admin.clusters.tlsHint']()}>
			{#if tlsInfo?.caSet}
				<p class="text-sm text-muted-foreground">
					{m['admin.clusters.caCurrent']({ subject: tlsInfo.caSubject ?? '', expires: showDate(tlsInfo.caNotAfter) })}
				</p>
				<Checkbox label={m['admin.clusters.caRemove']()} checked={clearCa} onToggle={(checked) => (clearCa = checked)} />
			{/if}
			<FormField label={m['admin.clusters.caPem']()} hint={m['admin.clusters.caPemHint']()}>
				{#snippet children({ id, describedBy, invalid })}
					<Textarea {id} {describedBy} {invalid} mono rows={4} bind:value={caPem} placeholder="-----BEGIN CERTIFICATE-----" />
				{/snippet}
			</FormField>
			<FormField label={m['admin.clusters.nodeFingerprints']()} hint={m['admin.clusters.nodeFingerprintsHint']()}>
				{#snippet children({ id, describedBy, invalid })}
					<Textarea {id} {describedBy} {invalid} mono rows={2} bind:value={nodeFingerprints} placeholder="AA:BB:CC:..." />
				{/snippet}
			</FormField>
			{#if tlsInfo?.clientCertSet}
				<p class="text-sm text-muted-foreground">
					{m['admin.clusters.clientCurrent']({ subject: tlsInfo.clientCertSubject ?? '', expires: showDate(tlsInfo.clientCertNotAfter) })}
				</p>
				<Checkbox label={m['admin.clusters.clientRemove']()} checked={clearClient} onToggle={(checked) => (clearClient = checked)} />
			{/if}
			<div class="grid gap-4 sm:grid-cols-2">
				<FormField label={m['admin.clusters.clientCertPem']()}>
					{#snippet children({ id, describedBy, invalid })}
						<Textarea {id} {describedBy} {invalid} mono rows={4} bind:value={clientCertPem} />
					{/snippet}
				</FormField>
				<FormField label={m['admin.clusters.clientKeyPem']()} hint={m['admin.clusters.clientKeyHint']()}>
					{#snippet children({ id, describedBy, invalid })}
						<Textarea {id} {describedBy} {invalid} mono rows={4} bind:value={clientKeyPem} autocomplete="off" spellcheck={false} />
					{/snippet}
				</FormField>
			</div>
		</FormSection>

		{#if error}
			<Alert>{error}</Alert>
		{/if}
	</form>
	{#snippet footer()}
		<Button variant="secondary" onclick={onClose} disabled={saving}>{m['common.cancel']()}</Button>
		<Button type="submit" form={FORM_ID} loading={saving} disabled={saving}>{m['common.save']()}</Button>
	{/snippet}
</Dialog>
