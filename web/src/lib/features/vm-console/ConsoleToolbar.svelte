<script lang="ts">
	import { getConsoleContext } from './console.svelte';

	const store = getConsoleContext();

	async function pasteFromLocal(): Promise<void> {
		await store.pasteFromLocalClipboard();
	}

	async function copyToLocal(): Promise<void> {
		await store.copyFromVMToLocal();
	}
</script>

<div class="flex flex-wrap items-center gap-2" data-testid="vm-console-toolbar">
	<button
		type="button"
		class="rounded-md border border-border bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
		disabled={store.state !== 'connected'}
		onclick={() => store.toggleScale()}
		data-testid="vm-console-scale"
	>
		Scale: {store.scaleViewport ? 'On' : 'Off'}
	</button>

	<button
		type="button"
		class="rounded-md border border-border bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
		disabled={store.state !== 'connected'}
		onclick={() => store.sendCtrlAltDel()}
		data-testid="vm-console-ctrlaltdel"
	>
		Ctrl+Alt+Del
	</button>

	<button
		type="button"
		class="rounded-md border border-border bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
		disabled={store.state !== 'connected'}
		onclick={() => store.disconnect()}
		data-testid="vm-console-disconnect"
	>
		Disconnect
	</button>

	<button
		type="button"
		class="rounded-md border border-border bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
		disabled={store.state === 'connecting' || store.state === 'idle'}
		onclick={() => store.reconnect()}
		data-testid="vm-console-reconnect-btn"
	>
		Reconnect
	</button>

	<div class="mx-2 h-5 w-px bg-border"></div>

	<button
		type="button"
		class="rounded-md border border-border bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
		disabled={store.state !== 'connected'}
		onclick={pasteFromLocal}
		data-testid="vm-console-paste-to-vm"
		title="Paste from your clipboard into the VM"
	>
		Paste to VM
	</button>

	<button
		type="button"
		class="rounded-md border border-border bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
		disabled={store.clipboard.fromVM === ''}
		onclick={copyToLocal}
		data-testid="vm-console-copy-from-vm"
		title="Copy the VM's clipboard to your clipboard"
	>
		Copy from VM
	</button>

	{#if store.clipboard.fromVM}
		<span class="text-xs text-muted-foreground" data-testid="vm-console-clipboard-preview">
			VM clipboard: {store.clipboard.fromVM.slice(0, 50)}{store.clipboard.fromVM.length > 50 ? '…' : ''}
		</span>
	{/if}

	{#if store.error && store.state !== 'error'}
		<p class="text-xs text-destructive" data-testid="vm-console-clipboard-error">
			{store.error}
		</p>
	{/if}
</div>
