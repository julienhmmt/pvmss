<script lang="ts">
	import PolicyForm from './PolicyForm.svelte';
	import PolicyImportDialog from './PolicyImportDialog.svelte';
	import { buildPolicyExport, type AdminPolicy, type AdminPolicyPatch, type PolicyExportItem, type PolicyImportOutcome } from './policy.svelte';
	import type { ClusterOption } from '#lib/shared/clusters.js';
	import { m } from '#lib/paraglide/messages.js';
	import PageHeader from '#lib/shared/ui/PageHeader.svelte';
	import ClusterSelector from '#lib/shared/ui/ClusterSelector.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import EmptyState from '#lib/shared/ui/EmptyState.svelte';
	import ConfirmDialog from '#lib/shared/ui/ConfirmDialog.svelte';
	import QuotaOverridesList from './QuotaOverridesList.svelte';
	import type { QuotaOverride } from '#lib/features/quota-overrides/quota-overrides.js';

	interface Props {
		policy: AdminPolicy | null;
		loading: boolean;
		error: string | null;
		errorCode: string | null;
		saving: boolean;
		saveError: string | null;
		clusterOptions: ClusterOption[];
		cluster: string;
		overrides: QuotaOverride[];
		overridesError: string | null;
		onClusterChange: (value: string) => void;
		onLoad: () => void;
		onRetry: () => void;
		onSave: (patch: AdminPolicyPatch) => void;
		onImport: (clusters: string[], item: PolicyExportItem) => Promise<PolicyImportOutcome[] | null>;
	}

	let {
		policy,
		loading,
		error,
		errorCode,
		saving,
		saveError,
		clusterOptions,
		cluster,
		overrides,
		overridesError,
		onClusterChange,
		onLoad,
		onRetry,
		onSave,
		onImport
	}: Props = $props();

	let dirty = $state(false);
	let pendingCluster: string | null = $state(null);
	let switchDialogOpen = $state(false);
	let showImport = $state(false);
	// The form captures the server snapshot when it mounts, so an import has to
	// remount it to show the values it just wrote.
	let formKey = $state(0);

	function downloadExport(): void {
		if (policy === null) return;
		const url = URL.createObjectURL(new Blob([buildPolicyExport(policy)], { type: 'application/json' }));
		const a = document.createElement('a');
		a.href = url;
		a.download = `pvmss-policy-${cluster || 'cluster'}.json`;
		a.click();
		// Revoke after the browser has taken the blob: Firefox cancels the
		// download when the URL dies in the same tick.
		setTimeout(() => URL.revokeObjectURL(url));
	}

	async function handleImport(clusters: string[], item: PolicyExportItem): Promise<PolicyImportOutcome[] | null> {
		const outcomes = await onImport(clusters, item);
		if (outcomes !== null) formKey++;
		return outcomes;
	}

	function handleDirtyChange(value: boolean): void {
		dirty = value;
	}

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

<svelte:head><title>{m['policy.title']()} - PVMSS</title></svelte:head>

<PageHeader cinematic title={m['policy.title']()} eyebrow={m['chrome.adminNav.policy']()} description={m['policy.description']()} titleId="policy-title">
	{#snippet actions()}
		<ClusterSelector
			options={clusterOptions}
			value={cluster}
			onChange={handleClusterChange}
			id="policy-cluster"
		/>
		<Button variant="secondary" disabled={policy === null} onclick={() => (showImport = true)}>{m['policy.import']()}</Button>
		<Button variant="secondary" disabled={policy === null} onclick={downloadExport}>{m['policy.export']()}</Button>
	{/snippet}
</PageHeader>

<section aria-labelledby="policy-title">
	{#if loading}
		<p class="text-muted-foreground" role="status" aria-live="polite">{m['policy.loading']()}</p>
	{:else if errorCode === 'inventory_not_ready'}
		<EmptyState
			title={m['policy.clusterUnreachableTitle']()}
			description={m['policy.clusterUnreachableDescription']()}
		>
			{#snippet actions()}
				<Button onclick={onRetry}>{m['policy.clusterUnreachableRetry']()}</Button>
			{/snippet}
		</EmptyState>
	{:else if error}
		<div class="space-y-3" role="alert">
			<p class="text-destructive">{error}</p>
			<Button variant="secondary" onclick={onLoad}>{m['policy.retry']()}</Button>
		</div>
	{:else if policy !== null}
		{#key formKey}
			<PolicyForm
				{policy}
				{saving}
				{saveError}
				{onSave}
				onDirtyChange={handleDirtyChange}
			>
				{#snippet quotaOverrides()}
					<QuotaOverridesList {overrides} error={overridesError} />
				{/snippet}
			</PolicyForm>
		{/key}
	{/if}
</section>

<PolicyImportDialog
	bind:open={showImport}
	{clusterOptions}
	{cluster}
	{saving}
	{saveError}
	onImport={handleImport}
	onClose={() => (showImport = false)}
/>

<ConfirmDialog
	open={switchDialogOpen}
	title={m['policy.switchClusterConfirmTitle']()}
	message={m['policy.switchClusterConfirmMessage']()}
	confirmLabel={m['policy.switchClusterConfirm']()}
	cancelLabel={m['common.cancel']()}
	onConfirm={confirmSwitch}
	onClose={closeSwitchDialog}
/>
