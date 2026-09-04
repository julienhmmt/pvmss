<script lang="ts">
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Button from '$lib/shared/ui/Button.svelte';

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
	<!-- Ticket 06: the old "applies on next reboot" line was false for most
	     fields — per-instance cloud-init modules do not replay while the
	     instance-id is unchanged. Say what applies when, per field group. -->
	<p id="cloudinit-save-description" class="mb-2 text-sm text-muted-foreground">
		{m['vms.cloudinit.dialogDescription']()}
	</p>
	<ul class="mb-4 grid gap-1.5 text-sm text-muted-foreground" data-testid="cloudinit-save-scopes">
		<li>{m['vms.cloudinit.dialogScopeNetwork']()}</li>
		<li>{m['vms.cloudinit.dialogScopeNow']()}</li>
		<li>{m['vms.cloudinit.dialogScopeFirstBoot']()}</li>
	</ul>
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
		<Button variant="secondary" onclick={close}>{m['common.cancel']()}</Button>
		<Button
			loading={saving}
			onclick={() => void confirm()}
			data-testid="cloudinit-save-confirm"
		>
			{saving ? m['common.saving']() : m['vms.cloudinit.dialogConfirm']()}
		</Button>
	</div>
</Dialog>
