<script lang="ts">
	/**
	 * RemoveApprovalDialog - confirms forgetting a stored catalog approval.
	 * Shared by the approval pages; the copy names the item. Removing an
	 * approval never touches Proxmox, so the copy stays factual.
	 */
	import ConfirmDialog from '#lib/shared/ui/ConfirmDialog.svelte';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		open: boolean;
		name: string;
		onConfirm: () => void;
		onClose: () => void;
	}

	let { open, name, onConfirm, onClose }: Props = $props();
</script>

<ConfirmDialog
	{open}
	title={m['admin.catalog.removeConfirmTitle']({ name })}
	message={m['admin.catalog.removeConfirmBody']({ name })}
	confirmLabel={m['admin.catalog.remove']()}
	cancelLabel={m['common.cancel']()}
	testId="catalog-remove-confirm"
	{onConfirm}
	{onClose}
/>
