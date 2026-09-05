<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import '@xterm/xterm/css/xterm.css';
	import { getSerialConsoleContext } from './serial.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Button from '$lib/shared/ui/Button.svelte';

	const store = getSerialConsoleContext();

	let container: HTMLDivElement;

	onMount(() => {
		void store.connect(container);
	});

	onDestroy(() => {
		store.disconnect();
	});
</script>

<div
	class="relative h-full w-full overflow-hidden rounded-md border border-border bg-black"
	data-testid="vm-serial-console-container"
>
	<div bind:this={container} class="h-full w-full"></div>

	{#if store.state === 'connecting'}
		<div
			class="absolute inset-0 flex items-center justify-center text-sm text-muted-foreground"
			data-testid="vm-serial-console-connecting"
		>
			{m['vms.console.connecting']()}
		</div>
	{/if}

	{#if store.state === 'error'}
		<div
			role="alert"
			aria-live="assertive"
			class="absolute inset-0 flex flex-col items-center justify-center gap-2 text-sm text-destructive"
			data-testid="vm-serial-console-error"
		>
			<p>{store.error ?? m['vms.console.connectionFailed']()}</p>
			<Button variant="secondary" size="sm" onclick={() => store.reconnect()} data-testid="vm-serial-console-retry">
				{m['vms.console.retry']()}
			</Button>
		</div>
	{/if}

	{#if store.state === 'disconnected'}
		<div
			role="status"
			aria-live="polite"
			class="absolute inset-0 flex flex-col items-center justify-center gap-2 text-sm text-muted-foreground"
			data-testid="vm-serial-console-disconnected"
		>
			<p>{m['vms.console.disconnected']()}</p>
			<Button variant="secondary" size="sm" onclick={() => store.reconnect()} data-testid="vm-serial-console-reconnect">
				{m['vms.console.reconnect']()}
			</Button>
		</div>
	{/if}
</div>
