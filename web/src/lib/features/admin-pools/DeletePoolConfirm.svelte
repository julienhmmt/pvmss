<script lang="ts">
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Dialog from '#lib/shared/ui/Dialog.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		open?: boolean;
		poolName: string;
		deleting: boolean;
		error: string | null;
		onClose: () => void;
		onConfirm: () => void;
	}

	let { open = $bindable(false), poolName, deleting, error, onClose, onConfirm }: Props = $props();

	// Pools are always created as "pvmss-<name>" (server PoolPrefix); show the name the admin typed.
	const name = $derived(poolName.replace(/^pvmss-/, ''));
</script>

<Dialog bind:open labelledBy="delete-pool-title" {onClose}>
	<h2 id="delete-pool-title" class="mb-2 text-lg font-semibold">{m['admin.pools.deleteTitle']({ name })}</h2>
	<p class="text-sm text-muted-foreground">
		{m['admin.pools.deleteConfirm']({ poolName })}
	</p>
	{#if error}
		<Alert class="mt-3">{error}</Alert>
	{/if}
	<div class="mt-6 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
		<Button variant="secondary" disabled={deleting} onclick={onClose}>{m['common.cancel']()}</Button>
		<Button variant="destructive" loading={deleting} onclick={onConfirm}>
			{deleting ? m['common.deleting']() : m['admin.pools.deletePoolLabel']({ name })}
		</Button>
	</div>
</Dialog>
