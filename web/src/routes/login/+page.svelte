<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import BrandPanel from '$lib/shared/ui/BrandPanel.svelte';
	import { LoginForm } from '$lib/features/auth/login.svelte';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import UserIcon from '$lib/shared/ui/icons/UserIcon.svelte';
	import LockIcon from '$lib/shared/ui/icons/LockIcon.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import { getStatusContext } from '$lib/features/chrome/status.svelte';

	const form = new LoginForm();
	const session = getSessionContext();
	const status = getStatusContext();

	$effect(() => {
		form.setPveDisabled(status.allClustersDown);
	});
	onMount(() => {
		void form.loadClusters();
	});

	async function submit(): Promise<void> {
		const principal = await form.submit();
		if (principal === null) return;
		// Reload the shell session so the signed-in sidebar appears without a
		// full page reload (the root layout gates the sidebar on principal).
		await session.load();
		await goto(resolve(principal.isAdmin ? '/admin' : '/'));
	}
</script>

<svelte:head>
	<title>{m['login.title']()}</title>
</svelte:head>

<div class="grid w-full flex-1 grid-cols-1 lg:grid-cols-2">
	<!-- Brand / marketing panel — desktop only (div, not aside, to avoid the
	     implicit complementary role — auth.spec.ts asserts 0 on /login) -->
	<BrandPanel />

	<!-- Login card panel -->
	<div class="flex flex-col items-center justify-center p-6 sm:p-10">
		<div class="w-full max-w-sm">
			<Card pad="lg" class="flex flex-col gap-4">
				<div>
					<h1 class="text-2xl font-semibold tracking-tight">{m['login.heading']()}</h1>
					<p class="mt-2 text-sm text-muted-foreground">{m['login.description']()}</p>
				</div>
				<form class="grid gap-4" onsubmit={(event) => { event.preventDefault(); void submit(); }}>
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
						<FormField label={m['login.username']()} hint={m['login.usernameRealmHint']()} required>
							{#snippet children({ id, describedBy, invalid })}
								<TextField
									{id}
									{describedBy}
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
					{/if}
					<FormField label={m['login.password']()} required>
						{#snippet children({ id, describedBy, invalid })}
							<TextField
								{id}
								{describedBy}
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
					{#if form.provider === 'pve' && form.selectedCluster?.oidcEnabled}
						<Button variant="secondary" onclick={() => void form.signInOIDC()} disabled={form.pveDisabled}>
							{m['login.signInOidc']()}
						</Button>
					{/if}
					{#if form.error}
						<p role="alert" class="text-sm text-destructive">{form.error}</p>
					{/if}
					<Button type="submit" loading={form.loading} disabled={form.provider === 'pve' && form.pveDisabled}>
						{form.loading ? m['login.signingIn']() : m['login.signIn']()}
					</Button>
				</form>
				<button
					type="button"
					class="self-start text-xs text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
					onclick={() => {
						form.provider = form.provider === 'pve' ? 'local' : 'pve';
						form.error = null;
					}}
				>
					{form.provider === 'pve' ? m['login.useLocalAdmin']() : m['login.backToProxmoxLogin']()}
				</button>
			</Card>
		</div>
	</div>
</div>
