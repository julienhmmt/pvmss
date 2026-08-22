<script lang="ts">
	import PolicyForm from './PolicyForm.svelte';
	import type { AdminPolicy, AdminPolicyPatch } from './policy.svelte';
	import type { ClusterOption } from '$lib/shared/clusters';
	import { m } from '$lib/paraglide/messages.js';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import ConfirmDialog from '$lib/shared/ui/ConfirmDialog.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';

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

	let {
		policy,
		loading,
		error,
		saving,
		saveError,
		saved,
		clusterOptions,
		cluster,
		onClusterChange,
		onLoad,
		onSave
	}: Props = $props();

	const toast = getToastContext();

	let dirty = $state(false);
	let pendingCluster: string | null = $state(null);
	let switchDialogOpen = $state(false);

	function handleDirtyChange(value: boolean): void {
		dirty = value;
	}

	// Fire a success toast when the store's `saved` flag is true. The store
	// resets `saved` to false at the start of each save, so this effect only
	// fires on the false→true transition.
	$effect(() => {
		if (saved) {
			toast.success(m['policy.saved']());
		}
	});

	function handleClusterChange(value: string): void {
		if (dirty) {
			pendingCluster = value;
			switchDialogOpen = true;
			return;
		}
		onClusterChange(value);
	}

	function confirmSwitch(): void {
		switchDialogOpen = false;
		if (pendingCluster !== null) {
			const next = pendingCluster;
			pendingCluster = null;
			onClusterChange(next);
		}
	}

	function closeSwitchDialog(): void {
		switchDialogOpen = false;
		pendingCluster = null;
	}
</script>

<svelte:head><title>{m['policy.title']()} — PVMSS</title></svelte:head>

<PageHeader title={m['policy.title']()} description={m['policy.description']()} titleId="policy-title">
	{#snippet actions()}
		<ClusterSelector
			options={clusterOptions}
			value={cluster}
			onChange={handleClusterChange}
			id="policy-cluster"
		/>
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
			<PolicyForm
				{policy}
				{saving}
				{saveError}
				{onSave}
				onDirtyChange={handleDirtyChange}
			/>
		{/key}
	{/if}
</section>

<ConfirmDialog
	open={switchDialogOpen}
	title={m['policy.switchClusterConfirmTitle']()}
	message={m['policy.switchClusterConfirmMessage']()}
	confirmLabel={m['policy.switchClusterConfirm']()}
	cancelLabel={m['common.cancel']()}
	onConfirm={confirmSwitch}
	onClose={closeSwitchDialog}
/>
