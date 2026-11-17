<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import BrandPanel from '#lib/shared/ui/BrandPanel.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import ButtonLink from '#lib/shared/ui/ButtonLink.svelte';
	import ClusterSelector from '#lib/shared/ui/ClusterSelector.svelte';
	import FormField from '#lib/shared/ui/FormField.svelte';
	import Select from '#lib/shared/ui/Select.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';
	import ChevronDownIcon from '#lib/shared/ui/icons/ChevronDownIcon.svelte';
	import { LoginForm, oidcErrorMessage } from '#lib/features/auth/login.svelte.js';
	import { getSessionContext } from '#lib/features/auth/session.svelte.js';
	import { getStatusContext } from '#lib/features/chrome/status.svelte.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		mode?: 'user' | 'admin';
	}

	const ERROR_ID = 'login-error';
	let { mode = 'user' }: Props = $props();
	const form = new LoginForm();
	const session = getSessionContext();
	const status = getStatusContext();
	let formEl: HTMLFormElement | null = $state(null);

	$effect(() => {
		form.provider = mode === 'admin' ? 'local' : 'pve';
		form.setPveDisabled(mode === 'user' && status.allClustersDown);
	});
	onMount(() => {
		if (mode === 'user') void form.loadClusters();
		// The SSO callback lands here on failure: /login?oidc=error&reason=<code>.
		if (mode === 'user' && page.url.searchParams.get('oidc') === 'error') {
			form.error = oidcErrorMessage(page.url.searchParams.get('reason'));
			void tick().then(() => document.getElementById(ERROR_ID)?.focus());
		}
	});
	const ssoUrl = $derived(mode === 'user' && !form.pveDisabled ? form.ssoUrl(page.url.searchParams.get('next')) : null);

	async function submit(): Promise<void> {
		const principal = await form.submit();
		if (principal === null) {
			await tick();
			focusAfterError();
			return;
		}
		// Reload the shell session: the root layout sees the principal and
		// sends the user on to `next` or their home.
		await session.load();
	}

	// Keyboard users land where they can act: the first empty field, or the
	// error itself when every field is filled.
	function focusAfterError(): void {
		const inputs = [...(formEl?.querySelectorAll<HTMLInputElement>('input:not([disabled])') ?? [])];
		(inputs.find((input) => input.value === '') ?? document.getElementById(ERROR_ID))?.focus();
	}

	function withError(describedBy: string | undefined): string | undefined {
		return form.error ? [describedBy, ERROR_ID].filter(Boolean).join(' ') : describedBy;
	}
</script>

<svelte:head>
	<title>{mode === 'admin' ? m['login.adminTitle']() : m['login.title']()}</title>
</svelte:head>

<div class="grid w-full flex-1 grid-cols-1 bg-background lg:grid-cols-2" data-testid={mode === 'admin' ? 'admin-login-screen' : 'user-login-screen'}>
	<!-- Brand / cinematic panel - desktop scene, compact brand header on mobile. -->
	<BrandPanel mode={mode === 'admin' ? 'admin' : 'login'} />

	<!-- Authentication panel -->
	<div class="flex flex-col justify-center px-6 py-10 sm:px-12 sm:py-14 lg:px-16 xl:px-24">
		<div class="mx-auto w-full max-w-[26rem]">
			<header class="mb-8">
				<p class="mb-3 text-xs font-semibold tracking-[0.16em] text-primary uppercase">
					{mode === 'admin' ? m['login.adminEyebrow']() : m['login.userEyebrow']()}
				</p>
				<h1 id="page-heading" tabindex="-1" class="text-3xl font-semibold tracking-[-0.035em] outline-none">
					{mode === 'admin' ? m['login.adminHeading']() : m['login.heading']()}
				</h1>
				<p class="mt-3 max-w-[42ch] text-sm leading-6 text-muted-foreground">
					{mode === 'admin' ? m['login.adminDescription']() : m['login.description']()}
				</p>
			</header>

			{#if mode === 'user' && form.clusters.length > 1}
				<div class="mb-7 border-b border-border pb-5 [&>div]:w-full [&>div]:justify-between">
					<ClusterSelector
						options={form.clusters}
						value={form.cluster}
						onChange={(value) => (form.cluster = value)}
						id="login-cluster"
						disabled={form.pveDisabled}
					/>
				</div>
			{/if}

			{#if mode === 'user' && form.pveDisabled}
				<p class="mb-6 rounded-md bg-warning-soft p-3 text-sm text-warning-soft-foreground" role="status">
					{m['login.clusterDownHint']()}
				</p>
			{/if}

			<form
				bind:this={formEl}
				id={mode === 'admin' ? 'admin-login-form' : 'user-login-form'}
				class="grid gap-5"
				onsubmit={(event) => { event.preventDefault(); void submit(); }}
			>
				{#if mode === 'user'}
					<FormField label={m['login.username']()} required>
						{#snippet children({ id, describedBy, invalid })}
							<TextField
								{id}
								describedBy={withError(describedBy)}
								{invalid}
								bind:value={form.username}
								autocomplete="username"
								disabled={form.pveDisabled}
								required
							/>
						{/snippet}
					</FormField>
				{:else}
					<FormField label={m['login.localUsername']()} hint={m['login.localUsernameHint']()}>
						{#snippet children({ id, describedBy })}
							<TextField
								{id}
								describedBy={withError(describedBy)}
								bind:value={form.localUsername}
								autocomplete="username"
								placeholder={m['login.usernamePlaceholder']()}
							/>
						{/snippet}
					</FormField>
				{/if}

				<FormField label={m['login.password']()} required>
					{#snippet children({ id, describedBy, invalid })}
						<TextField
							{id}
							describedBy={withError(describedBy)}
							{invalid}
							type="password"
							autocomplete="current-password"
							bind:value={form.password}
							reveal
							disabled={mode === 'user' && form.pveDisabled}
							required
						/>
					{/snippet}
				</FormField>

				{#if mode === 'user'}
					<details class="group -mt-1">
						<summary class="pv-focus flex w-fit cursor-pointer list-none items-center gap-2 rounded-[var(--radius-control)] text-xs font-medium text-muted-foreground transition-colors hover:text-foreground [&::-webkit-details-marker]:hidden">
							{m['login.advanced']()}
							<ChevronDownIcon class="h-3.5 w-3.5 transition-transform duration-150 group-open:rotate-180" />
						</summary>
						<div class="mt-4">
							<FormField label={m['login.realm']()} hint={m['login.realmHint']()}>
								{#snippet children({ id, describedBy })}
									<Select
										{id}
										describedBy={describedBy}
										bind:value={() => form.realm, (value) => (form.realm = value === 'pam' ? 'pam' : 'pve')}
										options={[
											{ value: 'pve', label: m['login.realmPve']() },
											{ value: 'pam', label: m['login.realmPam']() }
										]}
										disabled={form.pveDisabled}
									/>
								{/snippet}
							</FormField>
						</div>
					</details>
				{/if}

				{#if form.error}
					<Alert id={ERROR_ID} tabindex={-1}>{form.error}</Alert>
				{/if}
				<Button type="submit" size="lg" block loading={form.loading} disabled={mode === 'user' && form.pveDisabled} class="mt-1">
					{form.loading ? m['login.signingIn']() : m['login.signIn']()}
				</Button>
			</form>

			{#if ssoUrl}
				<div class="my-6 flex items-center gap-3" aria-hidden="true">
					<span class="h-px flex-1 bg-border"></span>
					<span class="text-xs text-muted-foreground">{m['login.sso.or']()}</span>
					<span class="h-px flex-1 bg-border"></span>
				</div>
				<ButtonLink href={ssoUrl} variant="secondary" size="lg" block data-sveltekit-reload data-testid="login-sso">
					{m['login.sso.button']()}
				</ButtonLink>
			{/if}

			<footer class="mt-9 border-t border-border pt-5 text-sm">
				{#if mode === 'admin'}
					<a href={resolve('/login')} class="pv-focus inline-flex rounded-[var(--radius-control)] font-medium text-muted-foreground transition-colors hover:text-foreground">
						<span aria-hidden="true" class="mr-2">←</span>{m['login.backToUser']()}
					</a>
				{:else}
					<div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
						<span class="text-muted-foreground">{m['login.adminLinkHint']()}</span>
						<a href={resolve('/admin/login')} class="pv-focus inline-flex rounded-[var(--radius-control)] font-medium text-primary underline-offset-4 hover:underline" data-testid="admin-login-link">
							{m['login.adminLink']()}<span aria-hidden="true" class="ml-1.5">→</span>
						</a>
					</div>
				{/if}
			</footer>
		</div>
	</div>
</div>
