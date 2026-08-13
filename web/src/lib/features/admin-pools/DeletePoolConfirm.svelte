<script lang="ts">
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import Button from '$lib/shared/ui/Button.svelte';

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
		<Button variant="ghost" onclick={onClose}>Cancel</Button>
		<Button variant="destructive" disabled={deleting} onclick={onConfirm}>
			{deleting ? 'Deleting…' : 'Delete pool'}
		</Button>
	</div>
</Dialog>
