<script lang="ts">
	import ClusterFormDialog from './ClusterFormDialog.svelte';
	import { statusHint, statusLabel, statusTone, type AdminCluster, type AdminClustersStore, type ClusterInput } from './clusters.svelte';
	import { resolve } from '$app/paths';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import PageHeader from '#lib/shared/ui/PageHeader.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import ButtonLink from '#lib/shared/ui/ButtonLink.svelte';
	import ConfirmDialog from '#lib/shared/ui/ConfirmDialog.svelte';
	import EmptyState from '#lib/shared/ui/EmptyState.svelte';
	import Pill from '#lib/shared/ui/Pill.svelte';
	import TableCard from '#lib/shared/ui/TableCard.svelte';
	import Pagination from '#lib/shared/ui/Pagination.svelte';
	import { createPagination } from '#lib/shared/ui/list-pager.svelte.js';
	const pager = createPagination('clusters', () => store.clusters);
	import TableSkeleton from '#lib/shared/ui/TableSkeleton.svelte';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		store: AdminClustersStore;
	}

	let { store }: Props = $props();
	let formOpen = $state(false);
	let editing = $state<AdminCluster | null>(null);
	let pendingDelete = $state<AdminCluster | null>(null);

	function openForm(cluster: AdminCluster | null): void {
		editing = cluster;
		formOpen = true;
	}

	async function confirmRemove(): Promise<void> {
		const cluster = pendingDelete;
		if (cluster === null) return;
		await store.remove(cluster.name);
		pendingDelete = null;
	}

	async function saveCluster(input: ClusterInput): Promise<void> {
		const succeeded =
			editing === null
				? await store.create(input)
				: await store.update(editing.name, {
						displayName: input.displayName ?? '',
						url: input.url,
						tlsInsecureSkipVerify: input.tlsInsecureSkipVerify,
						tokenId: input.tokenId,
						tokenSecret: input.tokenSecret,
						tls: input.tls
					});
		// Failure leaves the dialog open with store.error rendered inline
		// (ClusterFormDialog's error prop) - closing unconditionally here hid
		// create/update failures behind an easy-to-miss page-top banner.
		if (succeeded) {
			formOpen = false;
		}
	}
</script>

<PageHeader title={m['admin.clusters.heading']()} eyebrow={m['chrome.adminNav.infrastructure']()} description={m['admin.clusters.description']()}>
	{#snippet actions()}
		<Button onclick={() => void openForm(null)}>{m['admin.clusters.addCluster']()}</Button>
	{/snippet}
</PageHeader>

<section class="w-full">
	{#if store.announce}<p class="sr-only" role="status" aria-live="polite">{store.announce}</p>{/if}
	{#if store.error}<Alert class="mb-4">{store.error}</Alert>{/if}
	{#if store.loading}
		<div role="status" aria-live="polite" class="sr-only">{m['admin.clusters.loading']()}</div>
		<TableSkeleton columns={8} />
	{:else if store.clusters.length === 0}
		<EmptyState title={m['admin.clusters.empty']()}>
			{#snippet actions()}
				<Button onclick={() => void openForm(null)}>{m['admin.clusters.addCluster']()}</Button>
			{/snippet}
		</EmptyState>
	{:else}
		<TableCard>
			<table class="pv-table pv-responsive-table">
				<caption class="sr-only">{m['admin.clusters.caption']()}</caption>
				<thead>
					<tr>
						<th scope="col" class="font-medium">{m['common.name']()}</th>
						<th scope="col" class="font-medium">{m['common.status']()}</th>
						<th scope="col" class="font-medium">{m['admin.clusters.version']()}</th>
						<th scope="col" class="font-medium">{m['admin.clusters.nodesVms']()}</th>
						<th scope="col" class="font-medium">{m['common.actions']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each pager.rows as cluster (cluster.name)}
						<tr class="align-top">
							<td data-label={m['common.name']()}>
								<a class="font-medium hover:underline" href={resolve('/admin/clusters/[name]', { name: cluster.name })}>{cluster.displayName || cluster.name}</a>
								<div class="text-xs text-muted-foreground">{cluster.name}</div>
								<div class="max-w-xs truncate text-xs text-muted-foreground" title={cluster.url}>{cluster.url}</div>
							</td>
							<td data-label={m['common.status']()}>
								<Pill tone={statusTone(cluster.lastTestStatus)} label={statusLabel(cluster.lastTestStatus)} />
								{#if cluster.lastTestMessage}
									<div class="mt-1 text-xs text-muted-foreground">{cluster.lastTestMessage}</div>
								{:else if statusHint(cluster.lastTestStatus)}
									<div class="mt-1 text-xs text-muted-foreground">{statusHint(cluster.lastTestStatus)}</div>
								{/if}
							</td>
							<td class="text-muted-foreground" data-label={m['admin.clusters.version']()}>{cluster.proxmoxVersion ?? ' - '}</td>
							<td data-label={m['admin.clusters.nodesVms']()}>{cluster.nodeCount} / {cluster.vmCount}</td>

							<td data-label={m['common.actions']()}>
								<div class="flex flex-wrap gap-2">
									<ButtonLink variant="secondary" size="sm" href={resolve('/admin/clusters/[name]', { name: cluster.name })} label={m['admin.clusters.detailsLabel']({ name: cluster.name })}>{m['admin.clusters.details']()}</ButtonLink>
									<Button variant="secondary" size="sm" disabled={store.busy !== null} label={m['admin.clusters.testLabel']({ name: cluster.name })} onclick={() => void store.test(cluster.name)}>{m['admin.clusters.test']()}</Button>
									<Button variant="secondary" size="sm" disabled={store.busy !== null} label={m['admin.clusters.editLabel']({ name: cluster.name })} onclick={() => void openForm(cluster)}>{m['common.edit']()}</Button>
									<Button variant="destructive" size="sm" disabled={store.busy !== null} label={m['admin.clusters.removeLabel']({ name: cluster.name })} onclick={() => (pendingDelete = cluster)}>{m['admin.clusters.remove']()}</Button>
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#snippet footer()}
				<Pagination {pager} />
			{/snippet}
		</TableCard>
	{/if}
</section>

<ConfirmDialog
	open={pendingDelete !== null}
	title={m['admin.clusters.removeTitle']({ name: pendingDelete?.name ?? '' })}
	message={m['admin.clusters.removeConfirm']()}
	confirmLabel={m['admin.clusters.remove']()}
	cancelLabel={m['common.cancel']()}
	confirming={store.busy === `remove:${pendingDelete?.name}`}
	testId="cluster-remove-confirm"
	onConfirm={() => void confirmRemove()}
	onClose={() => (pendingDelete = null)}
/>

<ClusterFormDialog
	bind:open={formOpen}
	{editing}
	saving={store.busy === 'create' || store.busy === `update:${editing?.name}`}
	error={store.error}
	onClose={() => (formOpen = false)}
	onSubmit={saveCluster}
/>
