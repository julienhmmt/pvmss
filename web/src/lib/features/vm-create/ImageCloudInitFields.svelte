<script lang="ts">
	/**
	 * ImageCloudInitFields — the mandatory cloud-init section of image mode.
	 * Shared by the simple wizard and the detailed wizard's Base step.
	 * Delivered entirely through Proxmox's native cloud-init keys — the
	 * server cannot write a per-VM snippet file, so there is no packages or
	 * raw user-data field here; a fixed, admin-preplaced baseline snippet
	 * covers cluster-wide needs (e.g. installing qemu-guest-agent) instead.
	 * No password field: access is granted through SSH keys, and a password
	 * is set post-boot via the guest agent.
	 */
	import { getVmCreateContext } from './create.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Textarea from '$lib/shared/ui/Textarea.svelte';
	import Select from '$lib/shared/ui/Select.svelte';

	const form = getVmCreateContext();

	const ipModeOptions = [
		{ value: 'dhcp', label: m['vms.create.ciIpModeDhcp']() },
		{ value: 'static', label: m['vms.create.ciIpModeStatic']() }
	];

	const userError = $derived(form.ciUser.trim() === '' ? m['vms.create.errorCiUserRequired']() : null);
	const sshKeysError = $derived(
		form.sshKeys().length === 0 ? m['vms.create.errorCiSshKeysRequired']() : null
	);
	const ipAddressError = $derived(
		form.ciIpMode === 'static' && form.ciIpAddress.trim() === ''
			? m['vms.create.errorCiIpRequired']()
			: null
	);
</script>

<div class="grid gap-4 rounded-lg border border-border p-4" data-testid="image-cloud-init">
	<p class="text-sm font-medium text-foreground">{m['vms.create.cloudInitSection']()}</p>

	<FormField label={m['vms.create.ciUsername']()} required error={userError}>
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} bind:value={form.ciUser} required placeholder="admin" />
		{/snippet}
	</FormField>

	<FormField
		label={m['vms.create.ciSshKeys']()}
		required
		hint={m['vms.create.ciSshKeysHelp']()}
		error={sshKeysError}
	>
		{#snippet children({ id, describedBy, invalid })}
			<Textarea {id} {describedBy} {invalid} mono bind:value={form.ciSshKeysInput} rows={4} required placeholder="ssh-ed25519 AAAA..." />
		{/snippet}
	</FormField>

	<FormField label={m['vms.create.ciNetworkMode']()}>
		{#snippet children({ id, describedBy, invalid })}
			<Select {id} {describedBy} {invalid} bind:value={form.ciIpMode} options={ipModeOptions} />
		{/snippet}
	</FormField>

	{#if form.ciIpMode === 'static'}
		<div class="grid gap-4 sm:grid-cols-2">
			<FormField label={m['vms.create.ciIpAddress']()} required error={ipAddressError}>
				{#snippet children({ id, describedBy, invalid })}
					<TextField {id} {describedBy} {invalid} bind:value={form.ciIpAddress} required placeholder="192.168.1.50" />
				{/snippet}
			</FormField>
			<FormField label={m['vms.create.ciGateway']()} hint={m['common.optional']()}>
				{#snippet children({ id, describedBy, invalid })}
					<TextField {id} {describedBy} {invalid} bind:value={form.ciGateway} placeholder="192.168.1.1" />
				{/snippet}
			</FormField>
		</div>
	{/if}

	<p class="text-xs text-muted-foreground" data-testid="image-baseline-note">
		{m['vms.create.ciBaselineNote']()}
	</p>
</div>
