<script lang="ts">
	import { goto } from '$app/navigation';
	import { LoginForm } from '$lib/features/auth/login.svelte';

	const form = new LoginForm();

	async function submit(): Promise<void> {
		const principal = await form.submit();
		if (principal === null) return;
		await goto('/nodes');
	}
</script>

<svelte:head>
	<title>Sign in — PVMSS</title>
</svelte:head>

<section class="w-full max-w-sm rounded-lg border border-border bg-card p-6 shadow-sm">
	<h1 class="text-2xl font-semibold tracking-tight">Sign in to PVMSS</h1>
	<p class="mt-2 text-sm text-muted-foreground">Use a Proxmox account or the local administrator fallback.</p>
	<form class="mt-6 grid gap-4" onsubmit={(event) => { event.preventDefault(); void submit(); }}>
		<fieldset class="grid gap-2">
			<legend class="text-sm font-medium">Account type</legend>
			<label class="flex items-center gap-2 text-sm"><input type="radio" bind:group={form.provider} value="pve" /> Proxmox user</label>
			<label class="flex items-center gap-2 text-sm"><input type="radio" bind:group={form.provider} value="local" /> Local administrator</label>
		</fieldset>
		{#if form.provider === 'pve'}
			<label class="grid gap-1 text-sm font-medium">Username <input class="rounded-md border border-input bg-background px-3 py-2" autocomplete="username" bind:value={form.username} required /></label>
		{/if}
		<label class="grid gap-1 text-sm font-medium">Password <input class="rounded-md border border-input bg-background px-3 py-2" type="password" autocomplete="current-password" bind:value={form.password} required /></label>
		{#if form.error}<p role="alert" class="text-sm text-destructive">{form.error}</p>{/if}
		<button class="rounded-md bg-primary px-3 py-2 font-medium text-primary-foreground disabled:opacity-50" disabled={form.loading} type="submit">{form.loading ? 'Signing in…' : 'Sign in'}</button>
	</form>
	<p class="mt-5 text-xs text-muted-foreground">Demo user: alice / pvmss-alice</p>
</section>
