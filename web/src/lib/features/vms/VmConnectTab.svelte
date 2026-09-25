<script lang="ts">
	/**
	 * VmConnectTab - the default detail tab (DESIGN.md §6.3, "Connect"). It
	 * answers "how do I get in?": an SSH command built from the address the
	 * guest agent reports, or an honest "not available yet"; the real VNC
	 * console as the in-browser alternative; and the allocated resources.
	 * An address is never guessed and the SSH section only renders when the
	 * machine is running with a reported address (DESIGN.md §7).
	 */
	import { resolve } from '$app/paths';
	import { getVmDetailContext } from './detail.svelte';
	import { canConnect, type MachineDisplayStatus } from './display-status';
	import { primaryAddress, sshCommand } from './connection';
	import { compactBytes } from './machine-row';
	import { get } from '$lib/shared/api/client';
	import ButtonLink from '$lib/shared/ui/ButtonLink.svelte';
	import CopyButton from '$lib/shared/ui/CopyButton.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		status: MachineDisplayStatus;
	}

	let { status }: Props = $props();

	const store = getVmDetailContext();

	const address = $derived(primaryAddress(store.entity?.networkInterfaces));
	const connectable = $derived(canConnect(status, address?.address));

	// The cloud-init user is the account SSH lands on. Fetched once the
	// machine is connectable; unknown (template/ISO machines) stays unknown.
	let sshUser = $state<string | null>(null);
	let userLoaded = false;
	$effect(() => {
		if (!connectable || userLoaded) return;
		userLoaded = true;
		void get<{ user?: string }>(`/api/v1/vms/${encodeURIComponent(store.cluster)}/${store.vmid}/cloudinit`)
			.then((config) => {
				sshUser = config.user && config.user.trim() !== '' ? config.user.trim() : null;
			})
			.catch(() => {
				sshUser = null;
			});
	});

	const command = $derived(address ? sshCommand(sshUser, address.address) : '');
	const consoleHref = $derived(resolve('/vms/[cluster]/[vmid]/console', { cluster: store.cluster, vmid: String(store.vmid) }));
	const running = $derived(store.entity?.status === 'running');
</script>

{#if store.entity}
	{@const entity = store.entity}
	<div class="grid gap-5 min-[900px]:grid-cols-[minmax(0,1fr)_260px]">
		<section class="rounded-xl border border-border bg-card p-6 shadow-card" aria-labelledby="connect-ssh-title" data-testid="vm-connect-ssh">
			<p class="text-[0.6875rem] font-semibold uppercase tracking-[0.08em] text-muted-foreground-subtle">{m['vms.detail.connect.eyebrow']()}</p>
			{#if connectable && address}
				<h2 id="connect-ssh-title" class="mt-1 text-lg font-semibold">{m['vms.detail.connect.title']()}</h2>
				<p class="mt-1 text-sm text-muted-foreground">{m['vms.detail.connect.body']()}</p>
				<div class="mt-4 flex items-center gap-2 rounded-lg border border-border bg-muted/50 py-1.5 pl-3 pr-1.5">
					<code class="min-w-0 flex-1 overflow-x-auto whitespace-nowrap font-mono text-sm" data-testid="vm-ssh-command">
						<span class="select-none text-muted-foreground-subtle" aria-hidden="true">$&nbsp;</span>{command}
					</code>
					<CopyButton value={command} />
				</div>
				<p class="mt-2 text-xs text-muted-foreground">
					{m['vms.detail.connect.addressNote']()}
					{#if sshUser === null}{m['vms.detail.connect.userUnknown']()}{/if}
				</p>
				<dl class="mt-5 grid gap-x-6 gap-y-3 text-sm sm:grid-cols-3">
					<div>
						<dt class="text-xs text-muted-foreground">{m['vms.detail.connect.factUser']()}</dt>
						<dd class="mt-0.5 font-mono">{sshUser ?? m['vms.detail.connect.factUserUnknown']()}</dd>
					</div>
					<div>
						<dt class="text-xs text-muted-foreground">{m['vms.detail.connect.factAddress']()}</dt>
						<dd class="mt-0.5 font-mono" data-testid="vm-ssh-address">{address.address}</dd>
					</div>
					<div>
						<dt class="text-xs text-muted-foreground">{m['vms.detail.connect.factNetwork']()}</dt>
						<dd class="mt-0.5 font-mono">{address.bridge}</dd>
					</div>
				</dl>
				<p class="mt-5 border-t border-border pt-4 text-xs text-muted-foreground">{m['vms.detail.connect.helpNote']()}</p>
			{:else if running}
				<h2 id="connect-ssh-title" class="mt-1 text-lg font-semibold" data-testid="vm-address-unavailable">{m['vms.detail.connect.addressUnavailableTitle']()}</h2>
				<p class="mt-1 text-sm text-muted-foreground">{m['vms.detail.connect.addressUnavailableBody']()}</p>
				{#if entity.guestAgent === 'disabled'}
					<p class="mt-2 text-xs text-muted-foreground">{m['vms.detail.connect.agentDisabled']()}</p>
				{/if}
			{:else}
				<h2 id="connect-ssh-title" class="mt-1 text-lg font-semibold" data-testid="vm-ssh-unavailable">{m['vms.detail.connect.notRunningTitle']()}</h2>
				<p class="mt-1 text-sm text-muted-foreground">{m['vms.detail.connect.notRunningBody']()}</p>
			{/if}
		</section>

		<aside class="flex flex-col gap-3 rounded-xl border border-border bg-muted/40 p-5" aria-labelledby="connect-console-title">
			<h2 id="connect-console-title" class="text-sm font-semibold">{m['vms.detail.connect.consoleTitle']()}</h2>
			<p class="text-sm text-muted-foreground">{m['vms.detail.connect.consoleBody']()}</p>
			{#if running}
				<ButtonLink href={consoleHref} variant="secondary" block data-testid="vm-console-open">
					{m['vms.detail.connect.consoleOpen']()}
				</ButtonLink>
			{:else}
				<span
					class="inline-flex h-10 w-full cursor-not-allowed items-center justify-center rounded-[var(--radius-control)] border border-border bg-card text-sm font-semibold text-muted-foreground opacity-50"
					aria-disabled="true"
					data-testid="vm-console-disabled"
				>
					{m['vms.detail.connect.consoleOpen']()}
				</span>
			{/if}
			<p class="text-xs text-muted-foreground">{m['vms.detail.connect.consoleHint']()}</p>
		</aside>
	</div>

	<ul class="mt-5 grid gap-3 sm:grid-cols-3" aria-label={m['vms.detail.resource.caption']()} data-testid="vm-resource-strip">
		{#each [
			{ key: 'cpu', label: m['vms.detail.resource.cpu'](), value: String(entity.cpuCores) },
			{ key: 'memory', label: m['vms.detail.resource.memory'](), value: compactBytes(entity.memoryTotal) },
			{ key: 'storage', label: m['vms.detail.resource.storage'](), value: compactBytes(entity.diskTotal) }
		] as resource (resource.key)}
			<li class="flex flex-col gap-0.5 rounded-xl border border-border bg-card px-4 py-3" data-testid="vm-stat-{resource.key}">
				<span class="text-xs text-muted-foreground">{resource.label}</span>
				<span class="font-mono text-lg font-semibold tabular-nums">{resource.value}</span>
				<span class="text-[0.6875rem] text-muted-foreground-subtle">{m['vms.detail.resource.caption']()}</span>
			</li>
		{/each}
	</ul>
{/if}
