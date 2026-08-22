<script lang="ts">
	import type { CloudInitConfigUpdate, CloudInitIPMode, CloudInitStore } from './cloudinit.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Textarea from '$lib/shared/ui/Textarea.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';

	interface Props {
		store: CloudInitStore;
		onRequestSave: (update: CloudInitConfigUpdate) => void;
	}

	let { store, onRequestSave }: Props = $props();
	let user = $state('');
	let password = $state('');
	let sshKeys = $state('');
	let ipMode = $state<CloudInitIPMode>('dhcp');
	let ipAddress = $state('');
	let gateway = $state('');
	let dnsServer = $state('');
	let searchDomain = $state('');
	let validationError = $state<string | null>(null);
	let ipError = $state<string | null>(null);
	let gatewayError = $state<string | null>(null);
	let injectKey = $state('');
	let injectUser = $state('');

	$effect(() => {
		const config = store.config;
		if (config === null) return;
		user = config.user;
		sshKeys = config.sshKeys.join('\n');
		ipMode = config.ipMode;
		ipAddress = config.ipAddress ?? '';
		gateway = config.gateway ?? '';
		dnsServer = config.dnsServer ?? '';
		searchDomain = config.searchDomain ?? '';
	});

	function submit(): void {
	ipError = null;
	gatewayError = null;
	validationError = null;
	if (ipMode === 'static') {
		if (ipAddress === '') {
			ipError = m['vms.cloudinit.validationError']();
			return;
		}
		if (gateway === '') {
			gatewayError = m['vms.cloudinit.validationError']();
			return;
		}
	}
	onRequestSave({
		user,
		...(password === '' ? {} : { password }),
		sshKeys: sshKeys.split('\n').map((key) => key.trim()).filter(Boolean),
		ipMode,
		...(ipMode === 'static' ? { ipAddress, gateway } : {}),
		dnsServer,
		searchDomain
	});
	}

	async function injectNow(): Promise<void> {
	const key = injectKey.trim();
	if (key === '') return;
	const ok = await store.addSSHKey(key, injectUser);
	if (ok) {
		injectKey = '';
	}
	}
</script>

{#if store.configLoading && store.config === null}
	<div class="grid gap-3" role="status" aria-live="polite">
		<Skeleton class="h-4 w-24" />
		<Skeleton class="h-10 w-full" />
		<Skeleton class="h-4 w-32" />
		<Skeleton class="h-10 w-full" />
		<Skeleton class="h-4 w-20" />
		<Skeleton class="h-24 w-full" />
	</div>
{:else if store.configError && store.config === null}
	<p role="alert" data-testid="cloudinit-config-error">{store.configError}</p>
{:else}
	<form
		class="grid gap-4"
		aria-describedby="cloudinit-form-help"
		onsubmit={(event) => {
			event.preventDefault();
			submit();
		}}
	>
		<p id="cloudinit-form-help" class="text-sm text-muted-foreground">
			{m['vms.cloudinit.help']()}
		</p>
		<FormField label={m['vms.cloudinit.user']()} required>
			{#snippet children({ id, describedBy, invalid })}
				<TextField {id} {describedBy} {invalid} bind:value={user} data-testid="cloudinit-user" />
			{/snippet}
		</FormField>
		<FormField label={m['vms.cloudinit.password']()} hint={m['common.writeOnly']()}>
			{#snippet children({ id, describedBy, invalid })}
				<TextField {id} {describedBy} {invalid} type="password" bind:value={password} reveal data-testid="cloudinit-password" />
			{/snippet}
		</FormField>
		<FormField label={m['vms.cloudinit.sshKeys']()}>
			{#snippet children({ id, describedBy, invalid })}
				<Textarea {id} {describedBy} {invalid} mono rows={6} bind:value={sshKeys} onCmdEnter={submit} data-testid="cloudinit-ssh-keys" />
			{/snippet}
		</FormField>
		<div class="mt-2 rounded-lg border border-border bg-muted/30 p-3">
			<p class="text-sm font-medium">{m['vms.cloudinit.addSSHKey']()}</p>
			<p class="mt-1 text-xs text-muted-foreground">{m['vms.cloudinit.sshKeyHint']()}</p>
			<div class="mt-2 grid gap-2 sm:grid-cols-2">
				<TextField bind:value={injectKey} placeholder="ssh-ed25519 AAAA…" data-testid="cloudinit-inject-key" />
				<TextField bind:value={injectUser} placeholder={m['vms.cloudinit.sshKeyUser']()} data-testid="cloudinit-inject-user" />
			</div>
			<div class="mt-2 flex items-center gap-2">
				<Button type="button" loading={store.sshKeyInFlight} onclick={injectNow} data-testid="cloudinit-inject-now">
					{m['vms.cloudinit.addSSHKeyNow']()}
				</Button>
				{#if store.sshKeyError}
					<p role="alert" class="text-sm text-destructive">{store.sshKeyError}</p>
				{/if}
			</div>
		</div>
		<FormField label={m['vms.cloudinit.ipMode']()}>
			{#snippet children({ id, describedBy, invalid })}
				<Select
					{id}
					{describedBy}
					{invalid}
					bind:value={ipMode}
					options={[
						{ value: 'dhcp', label: m['vms.cloudinit.dhcp']() },
						{ value: 'static', label: m['vms.cloudinit.static']() }
					]}
					data-testid="cloudinit-ip-mode"
				/>
			{/snippet}
		</FormField>
		{#if ipMode === 'static'}
			<div class="grid gap-4 sm:grid-cols-2">
				<FormField label={m['vms.cloudinit.ipAddress']()} required error={ipError}>
					{#snippet children({ id, describedBy, invalid })}
						<TextField {id} {describedBy} {invalid} bind:value={ipAddress} required data-testid="cloudinit-ip-address" />
					{/snippet}
				</FormField>
				<FormField label={m['vms.cloudinit.gateway']()} required error={gatewayError}>
					{#snippet children({ id, describedBy, invalid })}
						<TextField {id} {describedBy} {invalid} bind:value={gateway} required data-testid="cloudinit-gateway" />
					{/snippet}
				</FormField>
			</div>
		{/if}
		<div class="grid gap-4 sm:grid-cols-2">
			<FormField label={m['vms.cloudinit.dnsServer']()}>
				{#snippet children({ id, describedBy, invalid })}
					<TextField {id} {describedBy} {invalid} bind:value={dnsServer} data-testid="cloudinit-dns" />
				{/snippet}
			</FormField>
			<FormField label={m['vms.cloudinit.searchDomain']()}>
				{#snippet children({ id, describedBy, invalid })}
					<TextField {id} {describedBy} {invalid} bind:value={searchDomain} data-testid="cloudinit-search-domain" />
				{/snippet}
			</FormField>
		</div>
		{#if validationError}
			<p role="alert" data-testid="cloudinit-validation-error">{validationError}</p>
		{/if}
		{#if store.configError}
			<p role="alert" data-testid="cloudinit-config-error">{store.configError}</p>
		{/if}
		<p class="sr-only" role="status" aria-live="polite">{store.configInFlight ? m['vms.cloudinit.saving']() : ''}</p>
		<Button type="submit" loading={store.configInFlight} data-testid="cloudinit-save">
			{store.configInFlight ? m['common.saving']() : m['vms.cloudinit.saveButton']()}
		</Button>
	</form>
{/if}
