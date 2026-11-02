<script lang="ts">
	import { getConsoleContext } from './console.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import Dialog from '#lib/shared/ui/Dialog.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import CopyButton from '#lib/shared/ui/CopyButton.svelte';

	interface Props {
		open: boolean;
		onClose: () => void;
	}

	let { open = $bindable(), onClose }: Props = $props();

	const store = getConsoleContext();

	// Held only while the dialog is open: never written to the store, never logged.
	let password = $state<string | null>(null);

	$effect(() => {
		if (open && password === null && !store.consolePasswordInFlight && store.consolePasswordError === null) {
			void generate();
		}
	});

	async function generate(): Promise<void> {
		password = await store.setConsolePassword();
	}

	function close(): void {
		password = null;
		onClose();
	}
</script>

<Dialog bind:open labelledBy="vm-console-password-title" onClose={close} size="sm">
	<h2 id="vm-console-password-title" class="text-lg font-semibold">{m['vms.console.passwordTitle']()}</h2>

	<div class="mt-3 flex flex-col gap-3 text-sm">
		<p>{m['vms.console.passwordBody']()}</p>

		{#if store.consolePasswordInFlight}
			<p class="text-muted-foreground" data-testid="vm-console-password-loading">
				{m['vms.console.passwordGenerating']()}
			</p>
		{:else if store.consolePasswordError}
			<p class="text-destructive" role="alert" data-testid="vm-console-password-error">{store.consolePasswordError}</p>
		{:else if password !== null}
			<div class="flex items-center gap-2">
				<code
					class="flex-1 rounded-[var(--radius-control)] border border-border bg-muted px-3 py-2 font-mono break-all"
					data-testid="vm-console-password-value">{password}</code
				>
				<CopyButton value={password} testId="vm-console-password-copy" />
			</div>
			<p class="text-xs text-muted-foreground">{m['vms.console.passwordOnce']()}</p>
		{/if}
	</div>

	<div class="mt-5 flex justify-end gap-2">
		{#if store.consolePasswordError}
			<Button variant="secondary" size="sm" onclick={() => void generate()} data-testid="vm-console-password-retry">
				{m['vms.console.retry']()}
			</Button>
		{/if}
		<Button variant="secondary" size="sm" onclick={close} data-testid="vm-console-password-close">
			{m['vms.console.passwordClose']()}
		</Button>
	</div>
</Dialog>
