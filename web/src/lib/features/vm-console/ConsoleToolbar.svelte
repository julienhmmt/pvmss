<script lang="ts">
	import { getConsoleContext } from './console.svelte';
	import { m } from '$lib/paraglide/messages.js';

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
		{m['vms.console.scale']()} {store.scaleViewport ? m['common.on']() : m['common.off']()}
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
		{m['vms.console.disconnect']()}
	</button>

	<button
		type="button"
		class="rounded-md border border-border bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
		disabled={store.state === 'connecting' || store.state === 'idle'}
		onclick={() => store.reconnect()}
		data-testid="vm-console-reconnect-btn"
	>
		{m['vms.console.reconnect']()}
	</button>

	<div class="mx-2 h-5 w-px bg-border"></div>

	<button
		type="button"
		class="rounded-md border border-border bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
		disabled={store.state !== 'connected'}
		onclick={pasteFromLocal}
		data-testid="vm-console-paste-to-vm"
		title={m['vms.console.pasteTitle']()}
	>
		{m['vms.console.pasteToVm']()}
	</button>

	<button
		type="button"
		class="rounded-md border border-border bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
		disabled={store.clipboard.fromVM === ''}
		onclick={copyToLocal}
		data-testid="vm-console-copy-from-vm"
		title={m['vms.console.copyTitle']()}
	>
		{m['vms.console.copyFromVm']()}
	</button>

	{#if store.clipboard.fromVM}
		<span class="text-xs text-muted-foreground" data-testid="vm-console-clipboard-preview">
			{m['vms.console.clipboardPreview']({ preview: store.clipboard.fromVM.slice(0, 50) })}{store.clipboard.fromVM.length > 50 ? '…' : ''}
		</span>
	{/if}

	{#if store.error && store.state !== 'error'}
		<p class="text-xs text-destructive" data-testid="vm-console-clipboard-error">
			{store.error}
		</p>
	{/if}
</div>
