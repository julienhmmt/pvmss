<script lang="ts">
	import { getConsoleContext } from './console.svelte';
	import { openConsolePopout } from './console';
	import { m } from '$lib/paraglide/messages.js';
	import Button from '$lib/shared/ui/Button.svelte';

	const store = getConsoleContext();

	let popoutBlocked = $state(false);

	function popout(): void {
		popoutBlocked = openConsolePopout(store.cluster, store.vmid) === null;
	}

	async function pasteFromLocal(): Promise<void> {
		await store.pasteFromLocalClipboard();
	}

	async function copyToLocal(): Promise<void> {
		await store.copyFromVMToLocal();
	}
</script>

<div class="flex flex-wrap items-center gap-2" data-testid="vm-console-toolbar">
	<Button
		variant="secondary"
		size="sm"
		disabled={store.state !== 'connected'}
		onclick={() => store.toggleScale()}
		data-testid="vm-console-scale"
	>
		{m['vms.console.scale']()} {store.scaleViewport ? m['common.on']() : m['common.off']()}
	</Button>

	<Button
		variant="secondary"
		size="sm"
		disabled={store.state !== 'connected'}
		onclick={() => store.sendCtrlAltDel()}
		data-testid="vm-console-ctrlaltdel"
	>
		Ctrl+Alt+Del
	</Button>

	<Button
		variant="secondary"
		size="sm"
		disabled={store.state !== 'connected'}
		onclick={() => store.disconnect()}
		data-testid="vm-console-disconnect"
	>
		{m['vms.console.disconnect']()}
	</Button>

	<Button
		variant="secondary"
		size="sm"
		disabled={store.state === 'connecting' || store.state === 'idle'}
		onclick={() => store.reconnect()}
		data-testid="vm-console-reconnect-btn"
	>
		{m['vms.console.reconnect']()}
	</Button>

	<Button variant="secondary" size="sm" onclick={popout} data-testid="vm-console-popout">
		{m['vms.console.popout']()}
	</Button>

	{#if popoutBlocked}
		<span class="text-xs text-destructive" data-testid="vm-console-popout-blocked">
			{m['vms.console.popoutBlocked']()}
		</span>
	{/if}

	<div class="mx-2 h-5 w-px bg-border"></div>

	<Button
		variant="secondary"
		size="sm"
		disabled={store.state !== 'connected'}
		onclick={pasteFromLocal}
		data-testid="vm-console-paste-to-vm"
		title={m['vms.console.pasteTitle']()}
	>
		{m['vms.console.pasteToVm']()}
	</Button>

	<Button
		variant="secondary"
		size="sm"
		disabled={store.clipboard.fromVM === ''}
		onclick={copyToLocal}
		data-testid="vm-console-copy-from-vm"
		title={m['vms.console.copyTitle']()}
	>
		{m['vms.console.copyFromVm']()}
	</Button>

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
