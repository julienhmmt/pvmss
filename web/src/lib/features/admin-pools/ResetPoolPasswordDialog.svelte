<script lang="ts">
	import type { ResetPoolCredentials } from './pool-detail.svelte';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Checkbox from '#lib/shared/ui/Checkbox.svelte';
	import CopyButton from '#lib/shared/ui/CopyButton.svelte';
	import Dialog from '#lib/shared/ui/Dialog.svelte';
	import FormField from '#lib/shared/ui/FormField.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';
	import { newPasswordError } from '#lib/shared/new-password.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		open?: boolean;
		username: string;
		saving: boolean;
		onClose: () => void;
		onReset: (password: string) => Promise<ResetPoolCredentials>;
	}

	let { open = $bindable(false), username, saving, onClose, onReset }: Props = $props();

	let customMode = $state(false);
	let password = $state('');
	let confirmation = $state('');
	let submitted = $state(false);
	let submitError = $state<string | null>(null);
	let credentials = $state<ResetPoolCredentials | null>(null);

	const passwordError = $derived(customMode ? newPasswordError(password, confirmation) : null);

	async function submit(): Promise<void> {
		submitted = true;
		submitError = null;
		if (passwordError !== null) return;
		try {
			credentials = await onReset(customMode ? password : '');
		} catch (err) {
			submitError = err instanceof Error ? err.message : m['admin.poolDetail.resetError']();
		}
	}
</script>

<Dialog bind:open labelledBy="pool-reset-title" {onClose}>
	{#if credentials}
		<h2 id="pool-reset-title" class="mb-2 text-lg font-semibold">{m['admin.poolDetail.resetDoneTitle']()}</h2>
		<Alert tone="warning" class="mb-4 inline-flex">{m['admin.poolDetail.resetDoneWarning']()}</Alert>
		<dl class="space-y-4">
			<div class="min-w-0">
				<dt class="mb-1 block text-sm font-medium">{m['admin.pools.username']()}</dt>
				<dd class="flex items-center gap-2">
					<code class="min-w-0 flex-1 break-all rounded-md border border-border bg-muted px-3 py-2 text-sm font-mono">{credentials.username}</code>
					<CopyButton variant="secondary" value={credentials.username} />
				</dd>
			</div>
			<div class="min-w-0">
				<dt class="mb-1 block text-sm font-medium">{m['admin.poolDetail.newPassword']()}</dt>
				<dd class="flex items-center gap-2">
					<code class="min-w-0 flex-1 break-all rounded-md border border-border bg-muted px-3 py-2 text-sm font-mono">{credentials.password}</code>
					<CopyButton variant="secondary" value={credentials.password} />
				</dd>
			</div>
		</dl>
		<div class="mt-5 flex justify-end">
			<Button onclick={onClose}>{m['admin.poolDetail.resetDoneDismiss']()}</Button>
		</div>
	{:else}
		<h2 id="pool-reset-title" class="mb-2 text-lg font-semibold">
			{m['admin.poolDetail.resetTitle']({ username })}
		</h2>
		<Alert tone="warning" class="mb-4 inline-flex">
			{m['admin.poolDetail.resetWarning']({ username })}
		</Alert>
		<form
			class="space-y-4"
			novalidate
			onsubmit={(event) => {
				event.preventDefault();
				void submit();
			}}
		>
			{#if submitError}
				<Alert>{submitError}</Alert>
			{/if}
			<Checkbox
				bind:checked={customMode}
				label={m['admin.poolDetail.resetCustom']()}
				data-testid="pool-reset-custom"
			/>
			{#if customMode}
				<FormField
					label={m['admin.poolDetail.newPassword']()}
					hint={m['admin.accounts.passwordHint']()}
					required
					error={submitted ? passwordError : null}
				>
					{#snippet children({ id, describedBy, invalid })}
						<TextField
							{id}
							{describedBy}
							{invalid}
							type="password"
							autocomplete="new-password"
							reveal
							bind:value={password}
							required
						/>
					{/snippet}
				</FormField>
				<FormField label={m['admin.accounts.passwordConfirm']()} required>
					{#snippet children({ id, describedBy })}
						<TextField
							{id}
							{describedBy}
							type="password"
							autocomplete="new-password"
							reveal
							bind:value={confirmation}
							required
						/>
					{/snippet}
				</FormField>
			{/if}
			<div class="flex justify-end gap-2 pt-2">
				<Button variant="secondary" disabled={saving} onclick={onClose}>{m['common.cancel']()}</Button>
				<Button type="submit" loading={saving} data-testid="pool-reset-submit">
					{m['admin.poolDetail.resetSubmit']()}
				</Button>
			</div>
		</form>
	{/if}
</Dialog>
