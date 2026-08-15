<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { LoginForm } from '$lib/features/auth/login.svelte';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const form = new LoginForm();
	const session = getSessionContext();
	onMount(() => {
		void form.loadClusters();
	});

	async function submit(): Promise<void> {
		const principal = await form.submit();
		if (principal === null) return;
		// Reload the shell session so the signed-in sidebar appears without a
		// full page reload (the root layout gates the sidebar on principal).
		await session.load();
		await goto(resolve('/nodes'));
	}
</script>

<svelte:head>
	<title>{m['login.title']()}</title>
</svelte:head>

<div class="grid w-full flex-1 grid-cols-1 lg:grid-cols-2">
	<!-- Brand / marketing panel — desktop only (div, not aside, to avoid the
	     implicit complementary role — auth.spec.ts asserts 0 on /login) -->
	<div class="relative hidden flex-col justify-between overflow-hidden bg-primary p-10 text-primary-foreground lg:flex">
		<div class="login-brand-glow absolute inset-0" aria-hidden="true"></div>
		<div class="relative flex flex-col gap-8">
			<a href={resolve('/')} class="text-lg font-bold tracking-tight">{m['shell.title']()}</a>
			<div class="max-w-md">
				<h2 class="text-3xl font-semibold leading-tight tracking-tight">{m['login.brandTagline']()}</h2>
				<p class="mt-3 text-sm text-primary-foreground/80">{m['shell.subtitle']()}</p>
			</div>
		</div>
		<ul class="relative flex flex-col gap-5">
			<li class="flex flex-col gap-1">
				<p class="text-sm font-semibold">{m['login.brandFeature1']()}</p>
				<p class="text-xs text-primary-foreground/75">{m['login.brandFeature1Desc']()}</p>
			</li>
			<li class="flex flex-col gap-1">
				<p class="text-sm font-semibold">{m['login.brandFeature2']()}</p>
				<p class="text-xs text-primary-foreground/75">{m['login.brandFeature2Desc']()}</p>
			</li>
			<li class="flex flex-col gap-1">
				<p class="text-sm font-semibold">{m['login.brandFeature3']()}</p>
				<p class="text-xs text-primary-foreground/75">{m['login.brandFeature3Desc']()}</p>
			</li>
		</ul>
	</div>

	<!-- Login card panel -->
	<div class="flex flex-col items-center justify-center p-6 sm:p-10">
		<div class="w-full max-w-sm">
			<Card pad="lg" class="flex flex-col gap-4">
				<div>
					<h1 class="text-2xl font-semibold tracking-tight">{m['login.heading']()}</h1>
					<p class="mt-2 text-sm text-muted-foreground">{m['login.description']()}</p>
				</div>
				<form class="grid gap-4" onsubmit={(event) => { event.preventDefault(); void submit(); }}>
					<fieldset class="grid gap-2">
						<legend class="text-sm font-medium">{m['login.accountType']()}</legend>
						<label class="flex items-center gap-2 text-sm">
							<input type="radio" bind:group={form.provider} value="pve" class="accent-primary" />
							{m['login.proxmoxUser']()}
						</label>
						<label class="flex items-center gap-2 text-sm">
							<input type="radio" bind:group={form.provider} value="local" class="accent-primary" />
							{m['login.localAdmin']()}
						</label>
					</fieldset>
					{#if form.provider === 'pve'}
						<ClusterSelector
							options={form.clusters}
							value={form.cluster}
							onChange={(value) => (form.cluster = value)}
							id="login-cluster"
						/>
						<label class="grid gap-1 text-sm font-medium">
							{m['login.username']()}
							<input
								class="rounded-[0.625rem] border border-input bg-background px-3 py-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
								autocomplete="username"
								bind:value={form.username}
								required
							/>
						</label>
					{/if}
					<label class="grid gap-1 text-sm font-medium">
						{m['login.password']()}
						<input
							class="rounded-[0.625rem] border border-input bg-background px-3 py-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
							type="password"
							autocomplete="current-password"
							bind:value={form.password}
							required
						/>
					</label>
					{#if form.provider === 'pve' && form.selectedCluster?.oidcEnabled}
						<button
							type="button"
							class="rounded-[0.625rem] border border-border bg-card px-3 py-2.5 text-sm font-semibold hover:bg-muted"
							onclick={() => void form.signInOIDC()}
						>
							{m['login.signInOidc']()}
						</button>
					{/if}
					{#if form.error}
						<p role="alert" class="text-sm text-destructive">{form.error}</p>
					{/if}
					<button
						class="rounded-[0.625rem] bg-primary px-3 py-2.5 font-semibold text-primary-foreground shadow-card transition-colors hover:bg-primary/90 disabled:opacity-50"
						disabled={form.loading}
						type="submit"
					>
						{form.loading ? m['login.signingIn']() : m['login.signIn']()}
					</button>
				</form>
				<p class="text-xs text-muted-foreground-subtle">{m['login.demoHint']()}</p>
			</Card>
		</div>
	</div>
</div>

<style>
	.login-brand-glow {
		background:
			radial-gradient(circle at 20% 20%, oklch(72% 0.17 44deg / 0.35), transparent 55%),
			radial-gradient(circle at 80% 75%, oklch(76% 0.16 75deg / 0.18), transparent 50%);
	}
</style>
