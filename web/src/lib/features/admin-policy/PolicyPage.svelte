<script lang="ts">
	import PolicyForm from './PolicyForm.svelte';
	import type { AdminPolicy, AdminPolicyPatch } from './policy.svelte';
	import type { ClusterOption } from '$lib/shared/clusters';
	import { m } from '$lib/paraglide/messages.js';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import Button from '$lib/shared/ui/Button.svelte';

	interface Props {
		policy: AdminPolicy | null;
		loading: boolean;
		error: string | null;
		saving: boolean;
		saveError: string | null;
		saved: boolean;
		clusterOptions: ClusterOption[];
		cluster: string;
		onClusterChange: (value: string) => void;
		onLoad: () => void;
		onSave: (patch: AdminPolicyPatch) => void;
	}

	let { policy, loading, error, saving, saveError, saved, clusterOptions, cluster, onClusterChange, onLoad, onSave }: Props = $props();
</script>

<svelte:head><title>{m['policy.title']()} — PVMSS</title></svelte:head>

<PageHeader title={m['policy.title']()} description={m['policy.description']()} titleId="policy-title">
	{#snippet actions()}
		<ClusterSelector options={clusterOptions} value={cluster} onChange={onClusterChange} id="policy-cluster" />
	{/snippet}
</PageHeader>

<section aria-labelledby="policy-title">
	{#if loading}
		<p class="text-muted-foreground" role="status" aria-live="polite">{m['policy.loading']()}</p>
	{:else if error}
		<div class="space-y-3" role="alert">
			<p class="text-destructive">{error}</p>
			<Button variant="secondary" onclick={onLoad}>{m['policy.retry']()}</Button>
		</div>
	{:else if policy !== null}
		{#key policy}
			<PolicyForm {policy} {saving} {saveError} {saved} {onSave} />
		{/key}
	{/if}
</section>
