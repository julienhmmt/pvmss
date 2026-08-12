<script lang="ts">
	import Dialog from '$lib/shared/ui/Dialog.svelte';

	interface Props {
		open?: boolean;
		poolName: string;
		deleting: boolean;
		error: string | null;
		onClose: () => void;
		onConfirm: () => void;
	}

	let { open = $bindable(false), poolName, deleting, error, onClose, onConfirm }: Props = $props();
</script>

<Dialog bind:open labelledBy="delete-pool-title" {onClose}>
	<h2 id="delete-pool-title" class="mb-3 text-lg font-semibold">Delete pool</h2>
	<p class="text-sm text-muted-foreground">
		This permanently deletes <strong class="text-foreground">{poolName}</strong>, its VMs, and its pool configuration.
	</p>
	{#if error}
		<p class="mt-3 text-sm text-destructive" role="alert">{error}</p>
	{/if}
	<div class="mt-6 flex justify-end gap-2">
		<button type="button" class="rounded-md px-4 py-2 text-sm text-muted-foreground hover:text-foreground" onclick={onClose}>Cancel</button>
		<button type="button" class="rounded-md bg-destructive px-4 py-2 text-sm font-medium text-destructive-foreground hover:bg-destructive/90" disabled={deleting} onclick={onConfirm}>
			{deleting ? 'Deleting…' : 'Delete pool'}
		</button>
	</div>
</Dialog>
