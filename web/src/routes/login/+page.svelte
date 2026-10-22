<script lang="ts">
	import { onMount, tick } from 'svelte';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import BrandPanel from '#lib/shared/ui/BrandPanel.svelte';
	import { LoginForm } from '#lib/features/auth/login.svelte.js';
	import { getSessionContext } from '#lib/features/auth/session.svelte.js';
	import ClusterSelector from '#lib/shared/ui/ClusterSelector.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import FormField from '#lib/shared/ui/FormField.svelte';
	import Select from '#lib/shared/ui/Select.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';
	import Tabs from '#lib/shared/ui/Tabs.svelte';
	import UserIcon from '#lib/shared/ui/icons/UserIcon.svelte';
	import LockIcon from '#lib/shared/ui/icons/LockIcon.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { getStatusContext } from '#lib/features/chrome/status.svelte.js';

	const form = new LoginForm();
	const session = getSessionContext();
	const status = getStatusContext();

	$effect(() => {
		form.setPveDisabled(status.allClustersDown);
	});
	onMount(() => {
		void form.loadClusters();
	});

	const ERROR_ID = 'login-error';
	let formEl: HTMLFormElement | null = $state(null);

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
	<title>{m['login.title']()}</title>
</svelte:head>

<div class="grid w-full flex-1 grid-cols-1 lg:grid-cols-2">
	<!-- Brand / marketing panel - desktop only (div, not aside, to avoid the
	     implicit complementary role - auth.spec.ts asserts 0 on /login) -->
	<BrandPanel />

	<!-- Login card panel -->
	<div class="flex flex-col items-center justify-center p-6 sm:p-10">
		<div class="w-full max-w-sm">
			<Card pad="lg" class="flex flex-col gap-4">
				<div>
					<h1 class="text-2xl font-semibold tracking-tight">{m['login.heading']()}</h1>
					<p class="mt-2 text-sm text-muted-foreground">{m['login.description']()}</p>
				</div>
				<Tabs
					tabs={[
						{ id: 'pve', label: m['login.proxmoxUser'] },
						{ id: 'local', label: m['login.localAdmin'] }
					]}
					bind:active={
						() => form.provider,
						(value) => {
							form.provider = value === 'local' ? 'local' : 'pve';
							form.error = null;
						}
					}
					class="grid grid-cols-2"
				/>
				<form
					bind:this={formEl}
					id={`panel-${form.provider}`}
					aria-labelledby={`tab-${form.provider}`}
					class="grid gap-4"
					onsubmit={(event) => { event.preventDefault(); void submit(); }}
				>
					{#if form.pveDisabled && form.provider === 'pve'}
						<p class="rounded-md bg-warning-soft p-3 text-sm text-warning-soft-foreground" role="status">
							{m['login.clusterDownHint']()}
						</p>
					{/if}
					{#if form.provider === 'pve'}
						<ClusterSelector
							options={form.clusters}
							value={form.cluster}
							onChange={(value) => (form.cluster = value)}
							id="login-cluster"
							disabled={form.pveDisabled}
						/>
						<div class="grid grid-cols-[1fr_auto] items-end gap-3">
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
									>
										{#snippet leading()}
											<UserIcon />
										{/snippet}
									</TextField>
								{/snippet}
							</FormField>
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
					{:else}
						<FormField
							label={m['login.localUsername']()}
							hint={m['login.localUsernameHint']()}
						>
							{#snippet children({ id, describedBy })}
								<TextField
									{id}
									describedBy={describedBy}
									bind:value={form.localUsername}
									autocomplete="username"
									placeholder="admin"
								>
									{#snippet leading()}
										<UserIcon />
									{/snippet}
								</TextField>
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
								disabled={form.provider === 'pve' && form.pveDisabled}
								required
							>
								{#snippet leading()}
									<LockIcon />
								{/snippet}
							</TextField>
						{/snippet}
					</FormField>
					{#if form.error}
						<Alert id={ERROR_ID} tabindex={-1}>{form.error}</Alert>
					{/if}
					<Button type="submit" loading={form.loading} disabled={form.provider === 'pve' && form.pveDisabled}>
						{form.loading ? m['login.signingIn']() : m['login.signIn']()}
					</Button>
				</form>
			</Card>
		</div>
	</div>
</div>
