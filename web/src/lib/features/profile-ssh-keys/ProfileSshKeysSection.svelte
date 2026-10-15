<script lang="ts">
	/**
	 * ProfileSshKeysSection - the "SSH keys" section of /profile: list the
	 * saved keys (label, fingerprint, creation date), add a new one, delete
	 * one behind a confirmation. Server errors are mapped to translated
	 * messages; the form is disabled once the profile is full (max 10).
	 */
	import { onMount } from 'svelte';
	import { m } from '$lib/paraglide/messages.js';
	import { ProfileSshKeysStore } from './profile-ssh-keys.svelte';
	import { profileKeyErrorMessage } from './ssh-key-identity';
	import { PROFILE_SSH_KEY_MAX, type ProfileSshKey } from './types';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Textarea from '$lib/shared/ui/Textarea.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import ConfirmDialog from '$lib/shared/ui/ConfirmDialog.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';

	const store = new ProfileSshKeysStore();

	let label = $state('');
	let publicKey = $state('');
	let adding = $state(false);
	let addError = $state<string | null>(null);
	let deleteError = $state<string | null>(null);
	let pendingDelete = $state<ProfileSshKey | null>(null);
	let deleting = $state(false);

	onMount(() => {
		void store.load();
	});

	/** Renders the API's RFC3339 timestamp in the user's locale; a malformed
	 *  value falls back to the raw string rather than "Invalid Date". */
	function formatCreated(value: string): string {
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return value;
		return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' }).format(date);
	}

	const canAdd = $derived(!adding && !store.limitReached && label.trim() !== '' && publicKey.trim() !== '');

	async function add(): Promise<void> {
		if (!canAdd) return;
		adding = true;
		addError = null;
		const result = await store.add(label.trim(), publicKey.trim());
		adding = false;
		if (result.ok) {
			label = '';
			publicKey = '';
			return;
		}
		addError = profileKeyErrorMessage(result.code);
	}

	async function confirmDelete(): Promise<void> {
		const key = pendingDelete;
		if (key === null || deleting) return;
		deleting = true;
		deleteError = null;
		const removed = await store.remove(key.id);
		deleting = false;
		pendingDelete = null;
		if (!removed) deleteError = m['profileSshKeys.deleteError']();
	}
</script>

<Card
	pad="none"
	class="mt-6"
	aria-labelledby="profile-ssh-keys-heading"
	data-testid="profile-ssh-keys"
>
	<div class="px-6 py-5">
		<h2 id="profile-ssh-keys-heading" class="font-medium">{m['profileSshKeys.heading']()}</h2>
		<p class="mt-1 text-sm text-muted-foreground">{m['profileSshKeys.body']()}</p>
		<p class="mt-2 text-xs text-muted-foreground">{m['profileSshKeys.help']()}</p>
	</div>

	{#if !store.loaded}
		<div class="grid gap-3 border-t border-border px-6 py-5" role="status" aria-live="polite">
			<Skeleton class="h-10 w-full" />
			<Skeleton class="h-10 w-full" />
		</div>
	{:else if store.loadError}
		<p class="border-t border-border px-6 py-4 text-sm text-destructive" role="alert" data-testid="ssh-keys-load-error">
			{store.loadError}
		</p>
	{:else if store.loading && store.keys.length === 0}
		<div class="grid gap-3 border-t border-border px-6 py-5" role="status" aria-live="polite">
			<Skeleton class="h-10 w-full" />
			<Skeleton class="h-10 w-full" />
		</div>
	{:else if store.keys.length === 0}
		<p class="border-t border-border px-6 py-4 text-sm text-muted-foreground" data-testid="ssh-keys-empty">
			{m['profileSshKeys.empty']()}
		</p>
	{:else}
		<ul>
			{#each store.keys as key (key.id)}
				<li
					class="flex flex-wrap items-center justify-between gap-3 border-t border-border px-6 py-4"
					data-testid="ssh-key-row-{key.id}"
				>
					<div class="min-w-0">
						<p class="truncate font-medium">{key.label}</p>
						<p class="break-all font-mono text-xs text-muted-foreground">{key.fingerprint}</p>
						<p class="text-xs text-muted-foreground-subtle">{formatCreated(key.createdAt)}</p>
					</div>
					<Button
						variant="ghost"
						aria-label={m['profileSshKeys.deleteLabel']({ label: key.label })}
						onclick={() => (pendingDelete = key)}
						data-testid="ssh-key-delete-{key.id}"
					>
						{m['common.delete']()}
					</Button>
				</li>
			{/each}
		</ul>
	{/if}

	{#if deleteError}
		<p class="border-t border-border px-6 py-3 text-sm text-destructive" role="alert" data-testid="ssh-keys-delete-error">
			{deleteError}
		</p>
	{/if}

	<div class="border-t border-border px-6 py-5">
		{#if store.limitReached}
			<p class="mb-3 text-sm text-muted-foreground" data-testid="ssh-keys-limit">
				{m['profileSshKeys.limitReached']({ max: PROFILE_SSH_KEY_MAX })}
			</p>
		{/if}
		<div class="grid gap-4">
			<FormField label={m['profileSshKeys.labelField']()} required>
				{#snippet children({ id, describedBy, invalid })}
					<TextField
						{id}
						{describedBy}
						{invalid}
						bind:value={label}
						required
						maxLength={64}
						disabled={store.limitReached}
						placeholder={m['profileSshKeys.labelPlaceholder']()}
						data-testid="ssh-key-add-label"
					/>
				{/snippet}
			</FormField>
			<FormField label={m['profileSshKeys.publicKeyField']()} required error={addError}>
				{#snippet children({ id, describedBy, invalid })}
					<!-- Explicit value+oninput rather than bind:value: equivalent
					     in the browser, and keeps the field usable in tests where
					     the autogrow action swallows the binding's input event. -->
					<Textarea
						{id}
						{describedBy}
						{invalid}
						mono
						rows={3}
						value={publicKey}
						oninput={(event: Event) => (publicKey = (event.currentTarget as HTMLTextAreaElement).value)}
						required
						disabled={store.limitReached}
						placeholder="ssh-ed25519 AAAA..."
						data-testid="ssh-key-add-key"
					/>
				{/snippet}
			</FormField>
			<div>
				<Button type="button" loading={adding} disabled={!canAdd} onclick={() => void add()} data-testid="ssh-key-add">
					{adding ? m['common.adding']() : m['profileSshKeys.add']()}
				</Button>
			</div>
		</div>
	</div>
</Card>

<ConfirmDialog
	open={pendingDelete !== null}
	title={m['profileSshKeys.deleteTitle']()}
	message={pendingDelete === null ? '' : m['profileSshKeys.deleteMessage']({ label: pendingDelete.label })}
	confirmLabel={m['common.delete']()}
	cancelLabel={m['common.cancel']()}
	confirming={deleting}
	testId="ssh-key-delete-confirm"
	onConfirm={() => void confirmDelete()}
	onClose={() => (pendingDelete = null)}
/>
