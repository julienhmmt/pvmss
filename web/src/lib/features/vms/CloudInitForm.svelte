<script lang="ts">
	import type { CloudInitConfigUpdate, CloudInitIPMode, CloudInitStore } from './cloudinit.svelte';

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
		validationError = validateForm();
		if (validationError !== null) return;
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

	function validateForm(): string | null {
		if (ipMode === 'static' && (ipAddress === '' || gateway === '')) {
			return 'Static IP mode requires an IP address and gateway.';
		}
		return null;
	}
</script>

{#if store.configLoading && store.config === null}
	<p role="status" aria-live="polite">Loading cloud-init configuration…</p>
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
			Changes apply after the VM reboots.
		</p>
		<label class="grid gap-1 text-sm">
			User
			<input class="rounded-md border border-border bg-background px-3 py-2" bind:value={user} data-testid="cloudinit-user" />
		</label>
		<label class="grid gap-1 text-sm">
			Password <span class="text-xs text-muted-foreground">write-only</span>
			<input class="rounded-md border border-border bg-background px-3 py-2" type="password" bind:value={password} data-testid="cloudinit-password" />
		</label>
		<label class="grid gap-1 text-sm">
			SSH public keys
			<textarea class="min-h-24 rounded-md border border-border bg-background px-3 py-2 font-mono text-xs" bind:value={sshKeys} data-testid="cloudinit-ssh-keys"></textarea>
		</label>
		<label class="grid gap-1 text-sm">
			IP mode
			<select class="rounded-md border border-border bg-background px-3 py-2" bind:value={ipMode} data-testid="cloudinit-ip-mode">
				<option value="dhcp">DHCP</option>
				<option value="static">Static</option>
			</select>
		</label>
		{#if ipMode === 'static'}
			<div class="grid gap-4 sm:grid-cols-2">
				<label class="grid gap-1 text-sm">
					IP address (CIDR)
					<input class="rounded-md border border-border bg-background px-3 py-2" required bind:value={ipAddress} data-testid="cloudinit-ip-address" />
				</label>
				<label class="grid gap-1 text-sm">
					Gateway
					<input class="rounded-md border border-border bg-background px-3 py-2" required bind:value={gateway} data-testid="cloudinit-gateway" />
				</label>
			</div>
		{/if}
		<div class="grid gap-4 sm:grid-cols-2">
			<label class="grid gap-1 text-sm">
				DNS server
				<input class="rounded-md border border-border bg-background px-3 py-2" bind:value={dnsServer} data-testid="cloudinit-dns" />
			</label>
			<label class="grid gap-1 text-sm">
				Search domain
				<input class="rounded-md border border-border bg-background px-3 py-2" bind:value={searchDomain} data-testid="cloudinit-search-domain" />
			</label>
		</div>
		{#if validationError}
			<p role="alert" data-testid="cloudinit-validation-error">{validationError}</p>
		{/if}
		{#if store.configError}
			<p role="alert" data-testid="cloudinit-config-error">{store.configError}</p>
		{/if}
		<p class="sr-only" role="status" aria-live="polite">{store.configInFlight ? 'Saving cloud-init configuration…' : ''}</p>
		<button type="submit" class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50" disabled={store.configInFlight} data-testid="cloudinit-save">
			{store.configInFlight ? 'Saving…' : 'Save configuration'}
		</button>
	</form>
{/if}
