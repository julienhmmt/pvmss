<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { setConsoleContext } from '$lib/features/vm-console/console.svelte';
	import { setSerialConsoleContext } from '$lib/features/vm-console/serial.svelte';
	import VmConsole from '$lib/features/vm-console/VmConsole.svelte';
	import VmSerialConsole from '$lib/features/vm-console/VmSerialConsole.svelte';
	import ConsoleToolbar from '$lib/features/vm-console/ConsoleToolbar.svelte';
	import { setVmDetailContext } from '$lib/features/vms/detail.svelte';
	import { m } from '$lib/paraglide/messages.js';

	type ConsoleMode = 'graphical' | 'text';

	const cluster = page.params.cluster ?? 'default';
	const vmid = Number(page.params.vmid);

	const store = setConsoleContext(cluster, vmid);
	const serialStore = setSerialConsoleContext(cluster, vmid);
	const vmStore = setVmDetailContext(cluster, vmid);

	let mode = $state<ConsoleMode>('graphical');

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
		void vmStore.load();
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
		<a
			href={resolve('/vms/[cluster]/[vmid]', { cluster, vmid: String(vmid) })}
			class="inline-flex items-center text-sm text-muted-foreground hover:text-foreground"
		>
			{m['common.backToVm']()}
		</a>
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

	<div class="mb-3 flex items-center gap-2" data-testid="vm-console-mode-switcher">
		<button
			type="button"
			class="rounded-md px-3 py-1.5 text-sm font-medium {mode === 'graphical'
				? 'bg-primary text-primary-foreground'
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
				? 'bg-primary text-primary-foreground'
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
		<div class="flex flex-wrap items-center gap-2" data-testid="vm-serial-console-toolbar">
			<button
				type="button"
				class="rounded-md border border-border bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
				disabled={serialStore.state !== 'connected'}
				onclick={() => serialStore.disconnect()}
				data-testid="vm-serial-console-disconnect"
			>
				{m['vms.console.disconnect']()}
			</button>
			<button
				type="button"
				class="rounded-md border border-border bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
				disabled={serialStore.state === 'connecting' || serialStore.state === 'idle'}
				onclick={() => serialStore.reconnect()}
				data-testid="vm-serial-console-reconnect-btn"
			>
				{m['vms.console.reconnect']()}
			</button>
		</div>
		{#if vmStore.entity && vmStore.entity.hasSerial === false}
			<div class="flex flex-wrap items-center gap-3 rounded-md border border-border bg-muted/40 p-3 text-sm" data-testid="vm-serial-console-enable">
				<p class="text-muted-foreground">{m['vms.console.serial.noSerial']()}</p>
				<button
					type="button"
					class="rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
					disabled={vmStore.serialEnabling}
					onclick={handleEnableSerial}
					data-testid="vm-serial-console-enable-btn"
				>
					{vmStore.serialEnabling ? m['vms.console.serial.enabling']() : m['vms.console.serial.enable']()}
				</button>
				{#if vmStore.serialEnableError}
					<p class="text-destructive" data-testid="vm-serial-console-enable-error">{vmStore.serialEnableError}</p>
				{/if}
			</div>
		{:else if vmStore.serialEnableError}
			<p class="text-destructive text-sm" data-testid="vm-serial-console-enable-error">{vmStore.serialEnableError}</p>
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
