<script lang="ts">
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		open?: boolean;
		saving: boolean;
		onConfirm: (rebootNow: boolean) => Promise<void>;
		onClose: () => void;
	}

	let { open = $bindable(false), saving, onConfirm, onClose }: Props = $props();
	let rebootNow = $state(false);

	async function confirm(): Promise<void> {
		await onConfirm(rebootNow);
	}

	function close(): void {
		rebootNow = false;
		onClose();
	}
</script>

<Dialog bind:open labelledBy="cloudinit-save-title" onClose={close}>
	<h2 id="cloudinit-save-title" class="mb-2 text-lg font-semibold">{m['vms.cloudinit.dialogTitle']()}</h2>
	<p id="cloudinit-save-description" class="mb-4 text-sm text-muted-foreground">
		{m['vms.cloudinit.dialogDescription']()}
	</p>
	<label class="flex items-start gap-2 text-sm">
		<input
			type="checkbox"
			class="mt-0.5 size-4 accent-primary"
			bind:checked={rebootNow}
			data-testid="cloudinit-reboot-checkbox"
		/>
		<span>{m['vms.cloudinit.dialogReboot']()}</span>
	</label>
	<div class="mt-6 flex justify-end gap-2">
		<button type="button" class="rounded-md border border-border px-4 py-2 text-sm hover:bg-muted" onclick={close}>
			{m['common.cancel']()}
		</button>
		<button
			type="button"
			class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
			disabled={saving}
			onclick={() => void confirm()}
			data-testid="cloudinit-save-confirm"
		>
			{saving ? m['common.saving']() : m['vms.cloudinit.dialogConfirm']()}
		</button>
	</div>
</Dialog>
