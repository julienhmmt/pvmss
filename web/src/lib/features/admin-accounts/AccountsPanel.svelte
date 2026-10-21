<script lang="ts">
	import { getAdminAccountsContext, type LocalAccount } from './accounts.svelte.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import ConfirmDialog from '#lib/shared/ui/ConfirmDialog.svelte';
	import Dialog from '#lib/shared/ui/Dialog.svelte';
	import EmptyState from '#lib/shared/ui/EmptyState.svelte';
	import FormField from '#lib/shared/ui/FormField.svelte';
	import Pill from '#lib/shared/ui/Pill.svelte';
	import TableCard from '#lib/shared/ui/TableCard.svelte';
	import TableSkeleton from '#lib/shared/ui/TableSkeleton.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';
	import { getToastContext } from '#lib/shared/ui/toast.svelte.js';
	import { formatDateTime } from '#lib/shared/format-date.js';
	import { m } from '#lib/paraglide/messages.js';

	const store = getAdminAccountsContext();
	const toast = getToastContext();

	const PASSWORD_MIN_LENGTH = 8;
	const PASSWORD_MAX_BYTES = 72;
	const USERNAME_MIN_LENGTH = 3;
	const USERNAME_MAX_LENGTH = 32;
	const USERNAME_PATTERN = /^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$/;

	// Create dialog
	let createOpen = $state(false);
	let createUsername = $state('');
	let createPassword = $state('');
	let createConfirm = $state('');
	let createSubmitted = $state(false);

	// Reset password dialog
	let resetTarget = $state<LocalAccount | null>(null);
	let resetPassword = $state('');
	let resetConfirm = $state('');
	let resetSubmitted = $state(false);

	// Confirmations
	let pendingToggle = $state<LocalAccount | null>(null);
	let pendingDelete = $state<LocalAccount | null>(null);

	const normalizedUsername = $derived(createUsername.trim().toLowerCase());

	const createUsernameError = $derived.by(() => {
		if (normalizedUsername === 'admin') return m['admin.accounts.errorUsernameReserved']();
		if (normalizedUsername.length < USERNAME_MIN_LENGTH || normalizedUsername.length > USERNAME_MAX_LENGTH) {
			return m['admin.accounts.errorUsernameLength']();
		}
		if (!USERNAME_PATTERN.test(normalizedUsername)) return m['admin.accounts.errorInvalidUsername']();
		return null;
	});

	const createPasswordError = $derived.by(() => passwordError(createPassword, createConfirm));
	const resetPasswordError = $derived.by(() => passwordError(resetPassword, resetConfirm));

	function passwordError(password: string, confirmation: string): string | null {
		if (password.length < PASSWORD_MIN_LENGTH) return m['admin.accounts.errorPasswordTooShort']();
		if (new TextEncoder().encode(password).length > PASSWORD_MAX_BYTES) {
			return m['admin.accounts.errorPasswordTooLong']();
		}
		if (password !== confirmation) return m['admin.accounts.errorPasswordMismatch']();
		return null;
	}

	function isLocked(account: LocalAccount): boolean {
		return account.lockedUntil !== undefined && new Date(account.lockedUntil).getTime() > Date.now();
	}

	function openCreate(): void {
		createUsername = '';
		createPassword = '';
		createConfirm = '';
		createSubmitted = false;
		store.clearSaveError();
		createOpen = true;
	}

	function closeCreate(): void {
		createOpen = false;
		store.clearSaveError();
	}

	async function submitCreate(): Promise<void> {
		createSubmitted = true;
		store.clearSaveError();
		if (createUsernameError !== null || createPasswordError !== null) return;
		const created = await store.create(normalizedUsername, createPassword);
		if (created) {
			createOpen = false;
			toast.success(m['admin.accounts.created']({ username: normalizedUsername }));
		}
	}

	function openReset(account: LocalAccount): void {
		resetTarget = account;
		resetPassword = '';
		resetConfirm = '';
		resetSubmitted = false;
		store.clearSaveError();
	}

	function closeReset(): void {
		resetTarget = null;
		store.clearSaveError();
	}

	async function submitReset(): Promise<void> {
		resetSubmitted = true;
		store.clearSaveError();
		if (resetTarget === null || resetPasswordError !== null) return;
		const target = resetTarget;
		const reset = await store.resetPassword(target.id, resetPassword);
		if (reset) {
			resetTarget = null;
			toast.success(m['admin.accounts.resetDone']({ username: target.username }));
		}
	}

	async function confirmToggle(): Promise<void> {
		const target = pendingToggle;
		if (target === null) return;
		pendingToggle = null;
		const disabled = !target.disabled;
		if (await store.setDisabled(target.id, disabled)) {
			toast.success(
				disabled
					? m['admin.accounts.disabledDone']({ username: target.username })
					: m['admin.accounts.enabledDone']({ username: target.username })
			);
		}
	}

	async function confirmDelete(): Promise<void> {
		const target = pendingDelete;
		if (target === null) return;
		pendingDelete = null;
		if (await store.remove(target.id)) {
			toast.success(m['admin.accounts.deletedDone']({ username: target.username }));
		}
	}
</script>

<section class="space-y-4" aria-label={m['admin.accounts.title']()}>
	{#if store.loading}
		<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
		<TableSkeleton columns={5} />
	{:else if store.error}
		<Alert>{store.error}</Alert>
	{:else}
		<div role="status" aria-live="polite" class="sr-only">
			{m['admin.accounts.loaded']({ count: store.count, max: store.max })}
		</div>

		{#if store.saveError && !createOpen && resetTarget === null}
			<Alert class="mb-4">{store.saveError}</Alert>
		{/if}

		{#if store.accounts.length > 0}
			<TableCard>
				{#snippet toolbar()}
					<p class="text-sm text-muted-foreground" data-testid="accounts-cap">
						{m['admin.accounts.cap']({ count: store.count, max: store.max })}
					</p>
					<Button class="ml-auto" onclick={openCreate} disabled={store.atLimit}>
						{m['admin.accounts.newAccount']()}
					</Button>
				{/snippet}
				<table class="pv-table pv-responsive-table">
					<caption class="sr-only">{m['admin.accounts.title']()}</caption>
					<thead>
						<tr>
							<th class="font-medium">{m['admin.accounts.username']()}</th>
							<th class="font-medium">{m['admin.accounts.status']()}</th>
							<th class="font-medium">{m['admin.accounts.lastLogin']()}</th>
							<th class="font-medium">{m['admin.accounts.createdAt']()}</th>
							<th class="font-medium">{m['common.actions']()}</th>
						</tr>
					</thead>
					<tbody>
						{#each store.accounts as account (account.id)}
							<tr class="transition-colors hover:bg-muted/40">
								<td class="font-mono text-xs" data-label={m['admin.accounts.username']()}>
									{account.username}
								</td>
								<td data-label={m['admin.accounts.status']()}>
									{#if account.disabled}
										<Pill tone="off" label={m['admin.accounts.statusDisabled']()} />
									{:else if isLocked(account)}
										<span class="inline-flex flex-wrap items-center gap-2">
											<Pill tone="warn" label={m['admin.accounts.statusLocked']()} />
											<span class="text-xs text-muted-foreground">
												{m['admin.accounts.lockedUntil']({ time: formatDateTime(account.lockedUntil ?? '', 'short') })}
											</span>
										</span>
									{:else}
										<Pill tone="ok" label={m['admin.accounts.statusActive']()} />
									{/if}
								</td>
								<td class="text-sm" data-label={m['admin.accounts.lastLogin']()}>
									{account.lastLoginAt
										? formatDateTime(account.lastLoginAt, 'short')
										: m['admin.accounts.never']()}
								</td>
								<td class="text-sm" data-label={m['admin.accounts.createdAt']()}>
									<time datetime={account.createdAt}>{formatDateTime(account.createdAt, 'short')}</time>
								</td>
								<td data-label={m['common.actions']()}>
									<div class="flex flex-wrap gap-2">
										<Button
											variant="secondary"
											size="sm"
											label={m['admin.accounts.resetLabel']({ username: account.username })}
											onclick={() => openReset(account)}
										>
											{m['admin.accounts.resetPassword']()}
										</Button>
										<Button
											variant="secondary"
											size="sm"
											label={account.disabled
												? m['admin.accounts.enableLabel']({ username: account.username })
												: m['admin.accounts.disableLabel']({ username: account.username })}
											onclick={() => (pendingToggle = account)}
										>
											{account.disabled ? m['admin.accounts.enable']() : m['admin.accounts.disable']()}
										</Button>
										<Button
											variant="destructive"
											size="sm"
											label={m['admin.accounts.deleteLabel']({ username: account.username })}
											onclick={() => (pendingDelete = account)}
										>
											{m['common.delete']()}
										</Button>
									</div>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</TableCard>
		{:else}
			<EmptyState title={m['admin.accounts.emptyTitle']()} description={m['admin.accounts.emptyBody']()}>
				{#snippet actions()}
					<Button onclick={openCreate}>{m['admin.accounts.newAccount']()}</Button>
				{/snippet}
			</EmptyState>
		{/if}
	{/if}
</section>

<Dialog bind:open={createOpen} labelledBy="account-create-title" onClose={closeCreate}>
	<h2 id="account-create-title" class="mb-2 text-lg font-semibold">{m['admin.accounts.newAccount']()}</h2>
	<p class="mb-4 text-sm text-muted-foreground">{m['admin.accounts.createBody']()}</p>
	<form
		class="space-y-4"
		novalidate
		onsubmit={(event) => {
			event.preventDefault();
			void submitCreate();
		}}
	>
		{#if store.saveError}
			<Alert>{store.saveError}</Alert>
		{/if}
		<FormField
			label={m['admin.accounts.username']()}
			hint={m['admin.accounts.usernameHint']()}
			required
			error={createSubmitted ? createUsernameError : null}
		>
			{#snippet children({ id, describedBy, invalid })}
				<TextField
					{id}
					{describedBy}
					{invalid}
					bind:value={createUsername}
					autocomplete="off"
					maxLength={USERNAME_MAX_LENGTH}
					required
				/>
			{/snippet}
		</FormField>
		<FormField
			label={m['admin.accounts.password']()}
			hint={m['admin.accounts.passwordHint']()}
			required
			error={createSubmitted ? createPasswordError : null}
		>
			{#snippet children({ id, describedBy, invalid })}
				<TextField
					{id}
					{describedBy}
					{invalid}
					type="password"
					autocomplete="new-password"
					reveal
					bind:value={createPassword}
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
					bind:value={createConfirm}
					required
				/>
			{/snippet}
		</FormField>
		<div class="flex justify-end gap-2 pt-2">
			<Button variant="ghost" onclick={closeCreate}>{m['common.cancel']()}</Button>
			<Button type="submit" loading={store.saving}>{m['admin.accounts.createAccount']()}</Button>
		</div>
	</form>
</Dialog>

<Dialog
	open={resetTarget !== null}
	labelledBy="account-reset-title"
	onClose={closeReset}
>
	<h2 id="account-reset-title" class="mb-2 text-lg font-semibold">
		{m['admin.accounts.resetTitle']({ username: resetTarget?.username ?? '' })}
	</h2>
	<p class="mb-4 text-sm text-muted-foreground">{m['admin.accounts.resetBody']()}</p>
	<form
		class="space-y-4"
		novalidate
		onsubmit={(event) => {
			event.preventDefault();
			void submitReset();
		}}
	>
		{#if store.saveError}
			<Alert>{store.saveError}</Alert>
		{/if}
		<FormField
			label={m['admin.accounts.password']()}
			hint={m['admin.accounts.passwordHint']()}
			required
			error={resetSubmitted ? resetPasswordError : null}
		>
			{#snippet children({ id, describedBy, invalid })}
				<TextField
					{id}
					{describedBy}
					{invalid}
					type="password"
					autocomplete="new-password"
					reveal
					bind:value={resetPassword}
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
					bind:value={resetConfirm}
					required
				/>
			{/snippet}
		</FormField>
		<div class="flex justify-end gap-2 pt-2">
			<Button variant="ghost" onclick={closeReset}>{m['common.cancel']()}</Button>
			<Button type="submit" loading={store.saving}>{m['admin.accounts.resetSubmit']()}</Button>
		</div>
	</form>
</Dialog>

<ConfirmDialog
	open={pendingToggle !== null}
	title={pendingToggle?.disabled
		? m['admin.accounts.enableTitle']({ username: pendingToggle.username })
		: m['admin.accounts.disableTitle']({ username: pendingToggle?.username ?? '' })}
	message={pendingToggle?.disabled
		? m['admin.accounts.enableBody']()
		: m['admin.accounts.disableBody']()}
	confirmLabel={pendingToggle?.disabled ? m['admin.accounts.enable']() : m['admin.accounts.disable']()}
	cancelLabel={m['common.cancel']()}
	confirming={store.saving}
	testId="account-toggle-confirm"
	onConfirm={() => void confirmToggle()}
	onClose={() => {
		pendingToggle = null;
		store.clearSaveError();
	}}
/>

<ConfirmDialog
	open={pendingDelete !== null}
	title={m['admin.accounts.deleteTitle']({ username: pendingDelete?.username ?? '' })}
	message={m['admin.accounts.deleteBody']()}
	confirmLabel={m['common.deletePermanently']()}
	cancelLabel={m['common.cancel']()}
	confirming={store.saving}
	testId="account-delete-confirm"
	onConfirm={() => void confirmDelete()}
	onClose={() => {
		pendingDelete = null;
		store.clearSaveError();
	}}
/>
