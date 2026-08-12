<script lang="ts">
	import PolicyForm from './PolicyForm.svelte';
	import type { AdminPolicy, AdminPolicyPatch } from './policy.svelte';
	import { resolveAdminPolicyCopy } from '$lib/i18n/admin-policy';

	interface Props {
		policy: AdminPolicy | null;
		loading: boolean;
		error: string | null;
		saving: boolean;
		saveError: string | null;
		saved: boolean;
		onLoad: () => void;
		onSave: (patch: AdminPolicyPatch) => void;
	}

	let { policy, loading, error, saving, saveError, saved, onLoad, onSave }: Props = $props();
	const copy = resolveAdminPolicyCopy();
</script>

<svelte:head><title>{copy.title} — PVMSS</title></svelte:head>

<section class="mx-auto w-full max-w-4xl px-4 py-8" aria-labelledby="policy-title">
	<div class="mb-8">
		<h1 id="policy-title" class="text-2xl font-semibold tracking-tight">{copy.title}</h1>
		<p class="mt-2 max-w-2xl text-sm text-muted-foreground">{copy.description}</p>
	</div>

	{#if loading}
		<p class="text-muted-foreground" role="status" aria-live="polite">{copy.loading}</p>
	{:else if error}
		<div class="space-y-3" role="alert">
			<p class="text-destructive">{error}</p>
			<button type="button" class="rounded-md border px-3 py-2 text-sm" onclick={onLoad}>{copy.retry}</button>
		</div>
	{:else if policy !== null}
		{#key policy}
			<PolicyForm {policy} {saving} {saveError} {saved} {onSave} />
		{/key}
	{/if}
</section>
