<script lang="ts">
	/**
	 * ConfirmDialog — a reusable destructive-action confirmation dialog.
	 * Wraps Dialog.svelte with a title, message, cancel button, and
	 * destructive confirm button. Used for delete/revoke actions that
	 * need an explicit confirmation step to prevent data loss.
	 */
	import Dialog from './Dialog.svelte';
	import Button from './Button.svelte';

	interface Props {
		open?: boolean;
		title: string;
		message: string;
		confirmLabel: string;
		cancelLabel: string;
		confirming?: boolean;
		testId?: string;
		onConfirm: () => void;
		onClose: () => void;
	}

	let {
		open = $bindable(false),
		title,
		message,
		confirmLabel,
		cancelLabel,
		confirming = false,
		testId,
		onConfirm,
		onClose
	}: Props = $props();

	const TITLE_ID = 'confirm-dialog-title';
</script>

<Dialog bind:open labelledBy={TITLE_ID} {onClose}>
	<h2 id={TITLE_ID} class="mb-2 text-lg font-semibold">{title}</h2>
	<p class="mb-4 text-sm text-muted-foreground">{message}</p>
	<div class="flex justify-end gap-2">
		<Button variant="ghost" onclick={onClose}>{cancelLabel}</Button>
		<Button
			variant="destructive"
			loading={confirming}
			onclick={onConfirm}
			data-testid={testId}
		>
			{confirmLabel}
		</Button>
	</div>
</Dialog>
