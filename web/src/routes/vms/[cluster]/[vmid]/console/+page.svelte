<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import Button from '#lib/shared/ui/Button.svelte';
	import { setConsoleContext } from '#lib/features/vm-console/console.svelte.js';
	import { setSerialConsoleContext } from '#lib/features/vm-console/serial.svelte.js';
	import VmConsole from '#lib/features/vm-console/VmConsole.svelte';
	import VmSerialConsole from '#lib/features/vm-console/VmSerialConsole.svelte';
	import ConsoleToolbar from '#lib/features/vm-console/ConsoleToolbar.svelte';
	import { setVmDetailContext } from '#lib/features/vms/detail.svelte.js';
	import VmActionBar from '#lib/features/vms/VmActionBar.svelte';
	import Pill from '#lib/shared/ui/Pill.svelte';
	import { getToastContext } from '#lib/shared/ui/toast.svelte.js';
	import { m } from '#lib/paraglide/messages.js';

	/** Live-status poll cadence while the console is open. */
	const LIVE_STATUS_POLL_MS = 5000;

	type ConsoleMode = 'graphical' | 'text';

	const cluster = page.params.cluster ?? '';
	const vmid = Number(page.params.vmid);

	const store = setConsoleContext(cluster, vmid);
	const serialStore = setSerialConsoleContext(cluster, vmid);
	const vmStore = setVmDetailContext(cluster, vmid);
	const toast = getToastContext();

	$effect(() => {
		if (vmStore.gone) {
			toast.info(m['vms.detail.goneRedirect']());
			void goto(resolve('/vms'));
		}
	});

	let mode = $state<ConsoleMode>('graphical');

	const stateLabels: Record<string, () => string> = {
		idle: () => m['vms.console.state.idle'](),
		connecting: () => m['vms.console.state.connecting'](),
		connected: () => m['vms.console.state.connected'](),
		disconnected: () => m['vms.console.state.disconnected'](),
		error: () => m['vms.console.state.error']()
	};

	function consoleStateLabel(state: string): string {
		return stateLabels[state]?.() ?? state;
	}

	function consoleStateTone(state: string): 'ok' | 'error' | 'off' {
		if (state === 'connected') return 'ok';
		if (state === 'error') return 'error';
		return 'off';
	}

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
		// The projection can be ~30s stale (VM just started): read the live
		// status once loaded, then keep it fresh while the console is open.
		const liveTimer = setInterval(() => void vmStore.refreshLiveStatus(), LIVE_STATUS_POLL_MS);
		void vmStore.load().then(() => void vmStore.refreshLiveStatus());
		// The connect happens inside VmConsole.svelte's onMount, which runs
		// after the container element is bound.
		return () => clearInterval(liveTimer);
	});

	onDestroy(() => {
		store.disconnect();
		serialStore.disconnect();
	});
</script>

<svelte:head>
	<title>{m['vms.console.title']({ name: vmStore.entity?.name || `VM ${vmid}` })}</title>
</svelte:head>

<section class="flex h-[calc(100vh-11rem)] min-h-[28rem] w-full flex-col">
	<div class="mb-3 flex flex-wrap items-center gap-3">
		<Button
			variant="secondary"
			size="sm"
			onclick={() => void goto(resolve('/vms/[cluster]/[vmid]', { cluster, vmid: String(vmid) }))}
		>
			{m['common.backToVm']()}
		</Button>
		<h1 id="page-heading" tabindex="-1" class="text-lg font-semibold focus:outline-none" data-testid="vm-console-title">
			{vmStore.entity?.name ? `${vmStore.entity.name} · ` : ''}{m['vms.console.heading']({ vmid: String(vmid) })}
		</h1>
		<span
			class="inline-flex items-center"
			aria-live="polite"
			data-testid="vm-console-status"
			data-state={mode === 'graphical' ? store.state : serialStore.state}
		>
			<Pill
				tone={consoleStateTone(mode === 'graphical' ? store.state : serialStore.state)}
				label={consoleStateLabel(mode === 'graphical' ? store.state : serialStore.state)}
			/>
		</span>
		<div class="ml-auto" data-testid="vm-console-action-bar">
			<VmActionBar hideDelete compact />
		</div>
	</div>

	<div class="flex flex-wrap items-center gap-2">
	<div class="inline-flex w-fit rounded-lg border border-border bg-muted/40 p-0.5" role="group" aria-label={m['vms.console.modeLabel']()} data-testid="vm-console-mode-switcher">
		<button
			type="button"
			class="rounded-md px-3 py-1.5 text-sm font-medium {mode === 'graphical'
				? 'bg-card text-foreground shadow-card'
				: 'text-muted-foreground hover:text-foreground'}"
			onclick={() => switchMode('graphical')}
			data-testid="vm-console-mode-graphical"
			aria-pressed={mode === 'graphical'}
		>
			{m['vms.console.mode.graphical']()}
		</button>
		<button
			type="button"
			class="rounded-md px-3 py-1.5 text-sm font-medium {mode === 'text'
				? 'bg-card text-foreground shadow-card'
				: 'text-muted-foreground hover:text-foreground'}"
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
			class="flex flex-1 flex-wrap items-center gap-2 rounded-[var(--radius-control)] border border-border bg-muted/40 px-2 py-1.5"
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
	{/if}
	</div>

	{#if mode === 'text'}
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

	{/if}

	<div class="mt-3 min-h-0 flex-1 overflow-hidden">
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
