<script lang="ts">
	/**
	 * VmConnectTab - the connect section of the detail Overview tab (DESIGN.md
	 * §6.3). It answers "how do I get in?": an SSH command built from the
	 * address the guest agent reports, or an honest "not available yet", and
	 * the browser console as the in-browser alternative. An address is never
	 * guessed and the SSH section only renders when the machine is running
	 * with a reported address (DESIGN.md §7). A stopped machine gets a single
	 * compact card with the Start button.
	 */
	import { resolve } from '$app/paths';
	import { getVmDetailContext } from './detail.svelte';
	import { canConnect, type MachineDisplayStatus } from './display-status';
	import { connectState, isWindowsOs, primaryAddress, sshCommand, sshHost } from './connection';
	import { get } from '#lib/shared/api/client.js';
	import Button from '#lib/shared/ui/Button.svelte';
	import ButtonLink from '#lib/shared/ui/ButtonLink.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import CopyButton from '#lib/shared/ui/CopyButton.svelte';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		status: MachineDisplayStatus;
		/** Starts the machine from the stopped card (wired to the page's power flow). */
		onStart?: () => void;
	}

	let { status, onStart }: Props = $props();

	const store = getVmDetailContext();

	const address = $derived(primaryAddress(store.entity?.networkInterfaces));
	const connect = $derived(connectState(status, store.entity ?? {}));
	const connectable = $derived(canConnect(status, address?.address) && connect === 'ready');
	const windows = $derived(isWindowsOs(store.entity?.ostype));

	// The cloud-init user is the account SSH lands on. Fetched once the
	// machine is connectable; unknown (template/ISO machines) stays unknown.
	// Windows guests log in with their own accounts - no fetch at all.
	let sshUser = $state<string | null>(null);
	let userLoaded = false;
	$effect(() => {
		if (!connectable || userLoaded || windows) return;
		userLoaded = true;
		void get<{ user?: string }>(`/api/v1/vms/${encodeURIComponent(store.cluster)}/${store.vmid}/cloudinit`)
			.then((config) => {
				sshUser = config.user && config.user.trim() !== '' ? config.user.trim() : null;
			})
			.catch(() => {
				sshUser = null;
			});
	});

	const userPlaceholder = $derived(`<${m['vms.detail.connect.userPlaceholder']()}>`);
	const command = $derived(address ? sshCommand(sshUser ?? userPlaceholder, address.address) : '');
	const consoleHref = $derived(resolve('/vms/[cluster]/[vmid]/console', { cluster: store.cluster, vmid: String(store.vmid) }));
	const running = $derived(store.entity?.status === 'running');
</script>

{#if store.entity}
	{#if connect === 'installing'}
		<Card pad="lg" aria-labelledby="connect-ssh-title" data-testid="vm-connect-ssh">
			<p class="text-2xs font-semibold uppercase tracking-[0.08em] text-muted-foreground-subtle">{m['vms.detail.connect.eyebrow']()}</p>
			<h2 id="connect-ssh-title" class="mt-1 text-lg font-semibold" data-testid="vm-installing">{m['vms.detail.connect.installingTitle']()}</h2>
			<p class="mt-1 text-sm text-muted-foreground">{m['vms.detail.connect.installingBody']()}</p>
			<div class="mt-4 flex flex-wrap items-center gap-3">
				<ButtonLink href={consoleHref} variant="primary" data-testid="vm-console-open">
					{m['vms.detail.connect.consoleOpen']()}
				</ButtonLink>
				<Button variant="secondary" loading={store.cdromInFlight} onclick={() => void store.setCdrom('disconnect')} data-testid="vm-eject-iso">
					{m['vms.detail.connect.ejectIso']()}
				</Button>
			</div>
			{#if store.writeError}<p class="mt-2 text-xs text-destructive" role="alert">{store.writeError}</p>{/if}
		</Card>
	{:else if running}
		<Card pad="lg" aria-labelledby="connect-ssh-title" data-testid="vm-connect-ssh">
			<div class="flex flex-wrap items-start justify-between gap-3">
				<div class="min-w-0">
					<p class="text-2xs font-semibold uppercase tracking-[0.08em] text-muted-foreground-subtle">{m['vms.detail.connect.eyebrow']()}</p>
					{#if connectable && address}
						<h2 id="connect-ssh-title" class="mt-1 text-lg font-semibold" data-testid={windows ? 'vm-rdp' : undefined}>{windows ? m['vms.detail.connect.rdpTitle']() : m['vms.detail.connect.title']()}</h2>
					{:else}
						<h2 id="connect-ssh-title" class="mt-1 text-lg font-semibold" data-testid="vm-address-unavailable">{m['vms.detail.connect.addressUnavailableTitle']()}</h2>
					{/if}
				</div>
				<ButtonLink href={consoleHref} variant="secondary" size="sm" data-testid="vm-console-open">
					{m['vms.detail.connect.consoleOpen']()}
				</ButtonLink>
			</div>
			{#if connectable && address}
				{#if windows}
					<p class="mt-1 text-sm text-muted-foreground">{m['vms.detail.connect.rdpBody']()}</p>
					<div class="mt-4 flex items-center gap-2 rounded-lg border border-border bg-muted/50 py-1.5 pl-3 pr-1.5">
						<code class="min-w-0 flex-1 overflow-x-auto whitespace-nowrap font-mono text-sm" data-testid="vm-rdp-address">
							{address.address}
						</code>
						<CopyButton value={address.address} />
					</div>
					<p class="mt-2 text-xs text-muted-foreground">{m['vms.detail.connect.addressNote']()}</p>
					<dl class="mt-5 grid gap-x-6 gap-y-3 text-sm sm:grid-cols-3">
						<div>
							<dt class="text-xs text-muted-foreground">{m['vms.detail.connect.factPort']()}</dt>
							<dd class="mt-0.5 font-mono">3389</dd>
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
					<p class="mt-5 border-t border-border pt-4 text-xs text-muted-foreground">{m['vms.detail.connect.rdpHelpNote']()}</p>
				{:else}
					<p class="mt-1 text-sm text-muted-foreground">{m['vms.detail.connect.body']()}</p>
					<div class="mt-4 flex items-center gap-2 rounded-lg border border-border bg-muted/50 py-1.5 pl-3 pr-1.5">
						<code class="min-w-0 flex-1 overflow-x-auto whitespace-nowrap font-mono text-sm" data-testid="vm-ssh-command">
							<span class="select-none text-muted-foreground-subtle" aria-hidden="true">$&nbsp;</span>{#if sshUser === null}ssh <span class="rounded bg-primary/10 px-1 italic text-primary" data-testid="vm-ssh-user-placeholder">{userPlaceholder}</span>@{sshHost(address.address)}{:else}{command}{/if}
						</code>
						<CopyButton value={command} />
					</div>
					<p class="mt-2 text-xs text-muted-foreground">
						{m['vms.detail.connect.addressNote']()}
						{#if sshUser === null}{m['vms.detail.connect.userUnknown']({ placeholder: userPlaceholder })}{/if}
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
				{/if}
			{:else}
				<p class="mt-1 text-sm text-muted-foreground">{m['vms.detail.connect.addressUnavailableBody']()}</p>
				{#if connect === 'agentDisabled'}
					<p class="mt-2 text-xs text-muted-foreground">{m['vms.detail.connect.agentDisabled']()}</p>
				{:else if connect === 'agentUnreachable'}
					<p class="mt-2 text-xs text-muted-foreground" data-testid="vm-agent-unreachable">{windows ? m['vms.detail.connect.agentUnreachableWindows']() : m['vms.detail.connect.agentUnreachable']()}</p>
				{:else}
					<p class="mt-2 text-xs text-muted-foreground">{m['vms.detail.connect.networkPending']()}</p>
				{/if}
			{/if}
		</Card>
	{:else}
		<!-- Stopped (or still provisioning): a single compact row - the
		     not-running message and the inline Start button. -->
		<Card pad="lg" aria-labelledby="connect-ssh-title" data-testid="vm-connect-ssh">
			<div class="flex flex-wrap items-center justify-between gap-4">
				<div class="min-w-0">
					<h2 id="connect-ssh-title" class="text-lg font-semibold" data-testid="vm-ssh-unavailable">{windows ? m['vms.detail.connect.rdpNotRunningTitle']() : m['vms.detail.connect.notRunningTitle']()}</h2>
					<p class="mt-1 text-sm text-muted-foreground">{m['vms.detail.connect.notRunningBody']()}</p>
				</div>
				{#if onStart}
					<Button
						loading={store.actionInFlight && store.inFlightActionKind === 'start'}
						disabled={store.actionInFlight || status === 'provisioning'}
						onclick={onStart}
						data-testid="vm-connect-start"
					>
						{m['vms.detail.power.start']()}
					</Button>
				{/if}
			</div>
		</Card>
	{/if}
{/if}
