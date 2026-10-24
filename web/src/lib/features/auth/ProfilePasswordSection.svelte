<script lang="ts">
	import { tick } from 'svelte';
	import { post, ApiRequestError } from '#lib/shared/api/client.js';
	import { getSessionContext } from './session.svelte';
	import { getToastContext } from '#lib/shared/ui/toast.svelte.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import FormField from '#lib/shared/ui/FormField.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';
	import { m } from '#lib/paraglide/messages.js';

	const MIN_PASSWORD_LENGTH = 8;

	const session = getSessionContext();
	const toast = getToastContext();

	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let currentTouched = $state(false);
	let newTouched = $state(false);
	let confirmTouched = $state(false);
	let submitted = $state(false);
	let submitting = $state(false);
	let submitError = $state<string | null>(null);
	let formEl: HTMLFormElement | undefined = $state();

	const isClusterAccount = $derived(session.principal?.provider === 'pve');
	const isLocalAccount = $derived(session.principal?.provider === 'local');
	const canChangePassword = $derived(isClusterAccount || isLocalAccount);

	const body = $derived(
		isLocalAccount ? m['account.password.bodyLocal']() : m['account.password.body']()
	);

	const rawCurrentError = $derived(
		currentPassword === '' ? m['account.password.errorCurrentRequired']() : null
	);
	const rawNewError = $derived(
		newPassword.length < MIN_PASSWORD_LENGTH ? m['account.password.errorTooShort']() : null
	);
	const rawConfirmError = $derived(
		confirmPassword !== newPassword ? m['account.password.errorMismatch']() : null
	);

	const currentError = $derived(submitted || currentTouched ? rawCurrentError : null);
	const newError = $derived(submitted || newTouched ? rawNewError : null);
	const confirmError = $derived(submitted || confirmTouched ? rawConfirmError : null);

	const valid = $derived(
		rawCurrentError === null && rawNewError === null && rawConfirmError === null
	);

	async function handleSubmit(): Promise<void> {
		submitted = true;
		submitError = null;
		if (!valid) {
			await tick();
			formEl?.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus();
			return;
		}
		submitting = true;
		try {
			await post<void>('/api/v1/auth/password', {
				oldPassword: currentPassword,
				newPassword
			});
			currentPassword = '';
			newPassword = '';
			confirmPassword = '';
			currentTouched = false;
			newTouched = false;
			confirmTouched = false;
			submitted = false;
			toast.success(m['account.password.success']());
		} catch (err) {
			if (err instanceof ApiRequestError && err.code === 'invalid_credentials') {
				submitError = m['account.password.errorInvalidCredentials']();
			} else {
				submitError = m['account.password.errorGeneric']();
			}
		} finally {
			submitting = false;
		}
	}
</script>

{#if session.principal}
	{#if canChangePassword}
		<Card
			pad="none"
			class="mt-6"
			aria-labelledby="profile-password-heading"
			data-testid="profile-password-section"
		>
			<div class="px-6 py-5">
				<h2 id="profile-password-heading" class="font-medium">{m['account.password.heading']()}</h2>
				<p class="mt-1 text-sm text-muted-foreground">{body}</p>

				<form
					bind:this={formEl}
					class="mt-4 grid gap-4 sm:max-w-md"
					novalidate
					onsubmit={(event) => {
						event.preventDefault();
						void handleSubmit();
					}}
				>
					<FormField label={m['account.password.current']()} required error={currentError}>
						{#snippet children({ id, describedBy, invalid })}
							<TextField
								{id}
								{describedBy}
								{invalid}
								type="password"
								autocomplete="current-password"
								reveal
								bind:value={currentPassword}
								onblur={() => (currentTouched = true)}
								required
								data-testid="profile-password-current"
							/>
						{/snippet}
					</FormField>

					<FormField
						label={m['account.password.new']()}
						hint={m['account.password.newHint']()}
						required
						error={newError}
					>
						{#snippet children({ id, describedBy, invalid })}
							<TextField
								{id}
								{describedBy}
								{invalid}
								type="password"
								autocomplete="new-password"
								reveal
								bind:value={newPassword}
								onblur={() => (newTouched = true)}
								required
								data-testid="profile-password-new"
							/>
						{/snippet}
					</FormField>

					<FormField label={m['account.password.confirm']()} required error={confirmError}>
						{#snippet children({ id, describedBy, invalid })}
							<TextField
								{id}
								{describedBy}
								{invalid}
								type="password"
								autocomplete="new-password"
								reveal
								bind:value={confirmPassword}
								onblur={() => (confirmTouched = true)}
								required
								data-testid="profile-password-confirm"
							/>
						{/snippet}
					</FormField>

					{#if submitError}
						<Alert data-testid="profile-password-error">{submitError}</Alert>
					{/if}

					<div>
						<Button type="submit" loading={submitting} data-testid="profile-password-submit">
							{submitting ? m['account.password.submitting']() : m['account.password.submit']()}
						</Button>
					</div>
				</form>
			</div>
		</Card>
	{:else}
		<Card pad="lg" class="mt-6" data-testid="profile-local-admin-note">
			<p class="text-sm text-muted-foreground">{m['account.password.localAdminNote']()}</p>
		</Card>
	{/if}
{/if}
