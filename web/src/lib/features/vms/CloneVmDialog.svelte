<script lang="ts">
	import Dialog from '#lib/shared/ui/Dialog.svelte';
	import FormField from '#lib/shared/ui/FormField.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import { ApiRequestError } from '#lib/shared/api/client.js';
	import { getTaskTrayContext } from '#lib/features/tasks/tasks.svelte.js';
	import { getToastContext } from '#lib/shared/ui/toast.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import { CLONE_NAME_PATTERN, cloneVm, suggestCopyName } from './clone';

	interface Props {
		open?: boolean;
		cluster: string;
		vmid: number;
		sourceName: string;
		/** Names already used in the caller's pool, so the suggestion skips them. */
		takenNames?: readonly string[];
	}

	let { open = $bindable(false), cluster, vmid, sourceName, takenNames = [] }: Props = $props();

	const tray = getTaskTrayContext();
	const toast = getToastContext();

	let name = $state('');
	let error = $state<string | null>(null);
	let inFlight = $state(false);

	// Re-seed the suggestion each time the dialog opens, so a reopened dialog
	// does not keep a stale name from a previous attempt.
	$effect(() => {
		if (open) {
			name = suggestCopyName(sourceName, takenNames);
			error = null;
		}
	});

	async function submit(): Promise<void> {
		if (inFlight) return;
		inFlight = true;
		error = null;
		const requested = name.trim();
		try {
			const accepted = await cloneVm(cluster, vmid, requested);
			tray.track({ upid: accepted.upid, kind: 'vm_clone', vmid: accepted.vmid, name: accepted.name, cluster: accepted.cluster });
			toast.info(m['vms.clone.queued']());
			open = false;
		} catch (caught: unknown) {
			error = caught instanceof ApiRequestError ? caught.message : m['vms.clone.errorOperation']();
		} finally {
			inFlight = false;
		}
	}

	function close(): void {
		error = null;
		open = false;
	}
</script>

<Dialog bind:open labelledBy="clone-vm-title" onClose={close}>
	<h2 id="clone-vm-title" class="mb-4 text-lg font-semibold">{m['vms.clone.title']()}</h2>
	<p class="mb-4 text-sm text-muted-foreground">{m['vms.clone.description']()}</p>
	<form onsubmit={(event) => { event.preventDefault(); void submit(); }}>
		<FormField label={m['vms.clone.nameLabel']()} hint={m['vms.clone.nameHint']()} required error={error}>
			{#snippet children({ id, describedBy, invalid })}
				<TextField
					{id}
					{describedBy}
					{invalid}
					bind:value={name}
					maxLength={63}
					required
					pattern={CLONE_NAME_PATTERN}
					data-testid="clone-name"
				/>
			{/snippet}
		</FormField>
		<div class="mt-6 flex justify-end gap-2">
			<Button type="button" variant="ghost" onclick={close}>{m['common.cancel']()}</Button>
			<Button type="submit" loading={inFlight} disabled={name.trim() === ''} data-testid="clone-confirm">
				{inFlight ? m['common.creating']() : m['vms.clone.confirm']()}
			</Button>
		</div>
	</form>
</Dialog>
