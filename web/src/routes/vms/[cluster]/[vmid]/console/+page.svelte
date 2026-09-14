<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import Button from '$lib/shared/ui/Button.svelte';
	import { setConsoleContext } from '$lib/features/vm-console/console.svelte';
	import { setSerialConsoleContext } from '$lib/features/vm-console/serial.svelte';
	import VmConsole from '$lib/features/vm-console/VmConsole.svelte';
	import VmSerialConsole from '$lib/features/vm-console/VmSerialConsole.svelte';
	import ConsoleToolbar from '$lib/features/vm-console/ConsoleToolbar.svelte';
	import { setVmDetailContext } from '$lib/features/vms/detail.svelte';
	import VmActionBar from '$lib/features/vms/VmActionBar.svelte';
	import { m } from '$lib/paraglide/messages.js';

	type ConsoleMode = 'graphical' | 'text';

	const cluster = page.params.cluster ?? '';
	const vmid = Number(page.params.vmid);

	const store = setConsoleContext(cluster, vmid);
	const serialStore = setSerialConsoleContext(cluster, vmid);
	const vmStore = setVmDetailContext(cluster, vmid);

	let mode = $state<ConsoleMode>('graphical');

	// cloud-image-console issue 06: image-born VMs (carrying the pvmss-image
	// tag) default to the text/serial tab - the graphical console under UEFI
	// renders as static for a cloud kernel, and SeaBIOS image VMs ship a
	// working text console from first boot. The user's manual switch still
	// wins; this only seeds the initial mode before the entity loads.
	const IMAGE_TAG = 'pvmss-image';

	function switchMode(next: ConsoleMode): void {
		if (mode === next) return;
		// Tear down the inactive session so no WebSocket leaks (the user
		// explicitly required clean teardown on mode switch).
		if (mode === 'graphical') {
			store.disconnect();
		} else {
			serialStore.disconnect();
		}
		mode = next;
	}

	async function handleEnableSerial(): Promise<void> {
		const ok = await vmStore.enableSerialConsole();
		if (ok) {
			serialStore.reconnect();
		}
	}

	onMount(() => {
		void vmStore.load().then(() => {
			// Issue 06: an image-born VM (pvmss-image tag) opens on the text
			// tab. Only seeds when the user has not yet switched - a manual
			// switch before load() lands keeps the user's choice.
			if (mode === 'graphical' && vmStore.entity?.tags?.includes(IMAGE_TAG)) {
				mode = 'text';
			}
		});
		// The connect happens inside VmConsole.svelte's onMount, which runs
		// after the container element is bound.
	});

	onDestroy(() => {
		store.disconnect();
		serialStore.disconnect();
	});
</script>

<svelte:head>
	<title>{m['vms.console.title']({ vmid: String(vmid) })}</title>
</svelte:head>

<section class="mx-auto flex h-screen w-full max-w-6xl flex-col px-4 py-4">
	<div class="mb-3 flex items-center gap-3">
		<Button
			variant="secondary"
			size="sm"
			onclick={() => void goto(resolve('/vms/[cluster]/[vmid]', { cluster, vmid: String(vmid) }))}
		>
			{m['common.backToVm']()}
		</Button>
		<h1 class="text-lg font-semibold" data-testid="vm-console-title">
			{m['vms.console.heading']({ vmid: String(vmid) })}
		</h1>
		<span
			class="inline-flex items-center rounded-full px-2 py-0.5 text-xs {mode === 'graphical'
				? store.state === 'connected'
					? 'bg-success-soft text-success-soft-foreground'
					: store.state === 'error'
						? 'bg-destructive-soft text-destructive-soft-foreground'
						: 'bg-muted text-muted-foreground'
				: serialStore.state === 'connected'
					? 'bg-success-soft text-success-soft-foreground'
					: serialStore.state === 'error'
						? 'bg-destructive-soft text-destructive-soft-foreground'
						: 'bg-muted text-muted-foreground'}"
			aria-live="polite"
			data-testid="vm-console-status"
		>
			{mode === 'graphical' ? store.state : serialStore.state}
		</span>
	</div>

	<div class="mb-3" data-testid="vm-console-action-bar">
		<VmActionBar hideDelete />
	</div>

	<div class="mb-3 flex items-center gap-2" data-testid="vm-console-mode-switcher">
		<button
			type="button"
			class="rounded-md px-3 py-1.5 text-sm font-medium {mode === 'graphical'
				? 'bg-primary-solid text-primary-foreground'
				: 'border border-border bg-background text-foreground hover:bg-muted'}"
			onclick={() => switchMode('graphical')}
			data-testid="vm-console-mode-graphical"
			aria-pressed={mode === 'graphical'}
		>
			{m['vms.console.mode.graphical']()}
		</button>
		<button
			type="button"
			class="rounded-md px-3 py-1.5 text-sm font-medium {mode === 'text'
				? 'bg-primary-solid text-primary-foreground'
				: 'border border-border bg-background text-foreground hover:bg-muted'}"
			onclick={() => switchMode('text')}
			data-testid="vm-console-mode-text"
			aria-pressed={mode === 'text'}
		>
			{m['vms.console.mode.text']()}
		</button>
	</div>

	{#if mode === 'graphical'}
		<ConsoleToolbar />
	{:else}
		<div
			class="flex flex-wrap items-center gap-2 rounded-[var(--radius-control)] border border-border bg-muted/40 px-2 py-1.5"
			data-testid="vm-serial-console-toolbar"
		>
			<Button
				variant="secondary"
				size="sm"
				disabled={serialStore.state !== 'connected'}
				onclick={() => serialStore.disconnect()}
				data-testid="vm-serial-console-disconnect"
				title={m['vms.console.disconnect']()}
			>
				{m['vms.console.disconnect']()}
			</Button>
			<Button
				variant={serialStore.state === 'connected' ? 'secondary' : 'primary'}
				size="sm"
				disabled={serialStore.state === 'connecting' || serialStore.state === 'idle'}
				onclick={() => serialStore.reconnect()}
				data-testid="vm-serial-console-reconnect-btn"
				title={m['vms.console.serial.reconnect']()}
			>
				{m['vms.console.serial.reconnect']()}
			</Button>
		</div>
		{#if vmStore.entity && vmStore.entity.hasSerial === false}
			<div class="flex flex-wrap items-center gap-3 rounded-md border border-border bg-muted/40 p-3 text-sm" data-testid="vm-serial-console-enable">
				<p class="text-muted-foreground">{m['vms.console.serial.noSerial']()}</p>
				<Button
					size="sm"
					disabled={vmStore.serialEnabling}
					onclick={handleEnableSerial}
					data-testid="vm-serial-console-enable-btn"
				>
					{vmStore.serialEnabling ? m['vms.console.serial.enabling']() : m['vms.console.serial.enable']()}
				</Button>
				{#if vmStore.serialEnableError}
					<p class="text-destructive" data-testid="vm-serial-console-enable-error">{vmStore.serialEnableError}</p>
				{/if}
			</div>
		{:else if vmStore.serialEnableError}
			<p class="text-destructive text-sm" data-testid="vm-serial-console-enable-error">{vmStore.serialEnableError}</p>
		{/if}
		<p class="text-xs text-muted-foreground" data-testid="vm-serial-console-hint">
			{m['vms.console.serial.hint']()}
		</p>

	{#if vmStore.entity?.tags?.includes(IMAGE_TAG) || vmStore.entity?.baselineState}
		<div class="mt-3 rounded-md border border-border bg-muted/40 p-3 text-sm" data-testid="vm-console-password-action">
			<p class="text-muted-foreground">{m['vms.console.setPasswordHint']()}</p>
			<div class="mt-2 flex flex-wrap items-center gap-2">
				<Button
					size="sm"
					disabled={vmStore.consolePasswordInFlight || vmStore.entity?.guestAgent === 'disabled' || vmStore.entity?.guestAgent === 'unreachable' || vmStore.entity?.status !== 'running'}
					onclick={() => void vmStore.setConsolePassword()}
					data-testid="vm-console-set-password-btn"
				>
					{vmStore.consolePasswordInFlight ? m['common.loading']() : m['vms.console.setPassword']()}
				</Button>
				{#if vmStore.entity?.guestAgent === 'disabled'}
					<p class="text-xs text-muted-foreground" data-testid="vm-console-password-agent-disabled">{m['vms.console.setPasswordAgentDisabled']()}</p>
				{:else if vmStore.entity?.guestAgent === 'unreachable' || (vmStore.entity?.status === 'running' && vmStore.entity?.guestAgent !== 'ok')}
					<p class="text-xs text-muted-foreground" data-testid="vm-console-password-disabled">{m['vms.console.setPasswordDisabled']()}</p>
				{/if}
			</div>
			{#if vmStore.consolePasswordError}
				<p class="mt-2 text-destructive text-sm" data-testid="vm-console-password-error">{vmStore.consolePasswordError}</p>
			{/if}
			{#if vmStore.generatedPassword}
				<div class="mt-2 rounded-md border border-success-soft bg-success-soft/40 p-2" data-testid="vm-console-password-generated">
					<p class="font-semibold text-sm">{m['vms.console.passwordGenerated']()}:</p>
					<code class="block mt-1 font-mono text-sm break-all">{vmStore.generatedPassword}</code>
					<p class="mt-1 text-xs text-muted-foreground">{m['vms.console.passwordCopyHint']()}</p>
				</div>
			{/if}
		</div>
	{/if}
	{/if}

	<div class="mt-3 flex-1 overflow-hidden">
		<svelte:boundary>
			{#if mode === 'graphical'}
				<VmConsole />
			{:else}
				<VmSerialConsole />
			{/if}
			{#snippet failed(error)}
				<div
					class="flex h-full w-full flex-col items-center justify-center gap-2 rounded-md border border-border bg-destructive-soft p-4 text-sm text-destructive"
					data-testid="vm-console-boundary-fallback"
				>
					<p>{m['vms.console.crashed']()}</p>
					<p class="text-xs text-muted-foreground">{error instanceof Error ? error.message : String(error)}</p>
					<button
						type="button"
						class="rounded-md border border-border bg-background px-3 py-1.5 text-sm text-foreground hover:bg-muted"
						onclick={() => window.location.reload()}
					>
						{m['common.reloadPage']()}
					</button>
				</div>
			{/snippet}
		</svelte:boundary>
	</div>
</section>
