<script lang="ts">
	/**
	 * SshKeyPicker - the shared "which SSH keys" control (spec D5/D7/D8/D9).
	 * Used by the VM creation wizards (image mode) and the VM cloud-init tab
	 * in add-only mode. Controlled: everything lives on the caller-owned
	 * SshKeySelection so the form can ask for finalKeys() / keyToSave().
	 *
	 * An empty profile degrades to the bare textarea - the same UX the
	 * SSH-key field always had (D7).
	 */
	import { m } from '$lib/paraglide/messages.js';
	import Textarea from '$lib/shared/ui/Textarea.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import type { SshKeySelection } from './ssh-key-selection.svelte';

	interface Props {
		selection: SshKeySelection;
		id?: string | undefined;
		describedBy?: string | undefined;
		invalid?: boolean;
	}

	// selection is $bindable so the picker's bind:value on selection.pasted /
	// selection.saveLabel compiles to writable bindings (the callers create
	// and own the instance; they only pass it one-way).
	let { selection = $bindable(), id, describedBy, invalid = false }: Props = $props();

	const offer = $derived(selection.saveOffer());
	// The help line only exists once the picker is more than the bare
	// textarea - an empty profile keeps the historical UX (D7).
	const showHelp = $derived(selection.profileKeys.length > 0 || offer !== null);
</script>

<div class="grid gap-3" data-testid="ssh-key-picker">
	{#if selection.profileKeys.length > 0}
		<div class="grid gap-2" role="group" aria-label={m['profileSshKeys.heading']()}>
			{#each selection.profileKeys as key (key.id)}
				{@const onVm = selection.isOnVm(key)}
				<label class="flex items-start gap-2 text-sm" data-testid="ssh-key-picker-option-{key.id}">
					<input
						type="checkbox"
						class="mt-0.5 h-4 w-4 rounded accent-primary pv-focus"
						checked={selection.isSelected(key.id)}
						disabled={onVm}
						onchange={(event) => selection.toggle(key.id, event.currentTarget.checked)}
					/>
					<span class="grid min-w-0 gap-0.5">
						<span class="font-medium text-foreground">{key.label}</span>
						<span class="break-all font-mono text-xs text-muted-foreground">{key.fingerprint}</span>
						{#if onVm}
							<span class="text-xs text-muted-foreground">{m['profileSshKeys.alreadyOnVm']()}</span>
						{/if}
					</span>
				</label>
			{/each}
		</div>
	{/if}

	<Textarea
		{id}
		{describedBy}
		{invalid}
		mono
		rows={4}
		bind:value={selection.pasted}
		placeholder="ssh-ed25519 AAAA..."
		data-testid="ssh-key-picker-paste"
	/>

	{#if offer !== null}
		<label class="flex items-start gap-2 text-sm">
			<input
				type="checkbox"
				class="mt-0.5 h-4 w-4 rounded accent-primary pv-focus"
				bind:checked={selection.saveToProfile}
				data-testid="ssh-key-picker-save"
			/>
			<span class="font-medium text-foreground">{m['profileSshKeys.saveToProfile']()}</span>
		</label>
		<TextField
			bind:value={selection.saveLabel}
			maxLength={64}
			data-testid="ssh-key-picker-save-label"
			aria-label={m['profileSshKeys.saveLabel']()}
		/>
	{/if}

	{#if showHelp}
		<p class="text-xs text-muted-foreground">{m['profileSshKeys.help']()}</p>
	{/if}
</div>
