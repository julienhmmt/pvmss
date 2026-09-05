<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { getConsoleContext } from './console.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Button from '$lib/shared/ui/Button.svelte';

	// Test-only: throws during component initialization to exercise the
	// <svelte:boundary> failed snippet in +page.svelte (SC-004). Svelte 5
	// boundaries only catch errors thrown during rendering or effects — not
	// event-handler or async errors. Since noVNC's async events are handled
	// gracefully by the store (state transitions, never throws), this is the
	// only reliable way to test that the boundary's failed snippet actually
	// catches a render-time throw.
	//
	// Deliberately NOT a URL query parameter: a flag readable from the URL is
	// bookmarkable, shareable, and crawlable — a real visitor could stumble
	// into a permanently broken console. Playwright sets this global via
	// page.addInitScript() before navigation, which only an already-privileged
	// test harness can do; no ordinary URL or link can trigger it. Still
	// present in the production bundle (constitution IX requires E2E against
	// the real built binary, not a stripped dev-only build), but the trigger
	// surface is now "can execute JS in the page before load," not "can guess
	// a query string." Type declared in test-hooks.d.ts.
	if (typeof window !== 'undefined' && window.__pvmssForceConsoleBoundaryError === true) {
		throw new Error('test-boundary: forced render-time error');
	}

	const store = getConsoleContext();

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
	data-testid="vm-console-container"
>
	<div bind:this={container} class="h-full w-full"></div>

	{#if store.state === 'connecting'}
		<div
			class="absolute inset-0 flex items-center justify-center text-sm text-muted-foreground"
			data-testid="vm-console-connecting"
		>
			{m['vms.console.connecting']()}
		</div>
	{/if}

	{#if store.state === 'error'}
		<div
			role="alert"
			aria-live="assertive"
			class="absolute inset-0 flex flex-col items-center justify-center gap-2 text-sm text-destructive"
			data-testid="vm-console-error"
		>
			<p>{store.error ?? m['vms.console.connectionFailed']()}</p>
			<Button variant="secondary" size="sm" onclick={() => store.reconnect()} data-testid="vm-console-retry">
				{m['vms.console.retry']()}
			</Button>
		</div>
	{/if}

	{#if store.state === 'disconnected'}
		<div
			role="status"
			aria-live="polite"
			class="absolute inset-0 flex flex-col items-center justify-center gap-2 text-sm text-muted-foreground"
			data-testid="vm-console-disconnected"
		>
			<p>{m['vms.console.disconnected']()}</p>
			<Button variant="secondary" size="sm" onclick={() => store.reconnect()} data-testid="vm-console-reconnect">
				{m['vms.console.reconnect']()}
			</Button>
		</div>
	{/if}
</div>
