<script lang="ts">
	import ClusterFormDialog from './ClusterFormDialog.svelte';
	import type { AdminCluster, AdminClustersStore, ClusterInput } from './clusters.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		store: AdminClustersStore;
	}

	let { store }: Props = $props();
	let formOpen = $state(false);
	let editing = $state<AdminCluster | null>(null);

	function addCluster(): void {
		editing = null;
		formOpen = true;
	}

	function editCluster(cluster: AdminCluster): void {
		editing = cluster;
		formOpen = true;
	}

	function statusLabel(status: AdminCluster['lastTestStatus']): string {
		switch (status) {
			case 'ok':
				return m['common.statusOk']();
			case 'unreachable':
				return m['common.statusUnreachable']();
			case 'error':
				return m['common.statusError']();
			default:
				return m['common.untested']();
		}
	}

	function statusClass(status: AdminCluster['lastTestStatus']): string {
		switch (status) {
			case 'ok':
				return 'bg-success text-success-foreground';
			case 'unreachable':
			case 'error':
				return 'bg-destructive text-destructive-foreground';
			default:
				return 'bg-muted text-muted-foreground';
		}
	}

	function statusHint(status: AdminCluster['lastTestStatus']): string | null {
		switch (status) {
			case 'unreachable':
				return m['admin.clusters.unreachableHint']();
			case 'error':
				return m['admin.clusters.errorHint']();
			default:
				return null;
		}
	}

	async function saveCluster(input: ClusterInput): Promise<void> {
		const succeeded =
			editing === null
				? await store.create(input)
				: await store.update(editing.name, {
						url: input.url,
						tlsInsecureSkipVerify: input.tlsInsecureSkipVerify,
						tokenId: input.tokenId,
						tokenSecret: input.tokenSecret
					});
		// Failure leaves the dialog open with store.error rendered inline
		// (ClusterFormDialog's error prop) — closing unconditionally here hid
		// create/update failures behind an easy-to-miss page-top banner.
		if (succeeded) formOpen = false;
	}
</script>

<PageHeader title={m['admin.clusters.heading']()} description={m['admin.clusters.description']()}>
	{#snippet actions()}
		<Button onclick={addCluster}>{m['admin.clusters.addCluster']()}</Button>
	{/snippet}
</PageHeader>

<section class="mx-auto w-full max-w-6xl">
	{#if store.announce}<p class="sr-only" role="status" aria-live="polite">{store.announce}</p>{/if}
	{#if store.error}<p class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive" role="alert">{store.error}</p>{/if}
	{#if store.loading}
		<div role="status" aria-live="polite" class="sr-only">{m['admin.clusters.loading']()}</div>
		<TableSkeleton columns={6} />
	{:else}
		<div class="overflow-x-auto rounded-lg border border-border">
			<table class="w-full min-w-[900px] text-left text-sm">
				<caption class="sr-only">{m['admin.clusters.caption']()}</caption>
				<thead class="bg-muted/50">
					<tr>
						<th scope="col" class="px-4 py-3 font-medium">{m['common.name']()}</th>
						<th scope="col" class="px-4 py-3 font-medium">{m['admin.clusters.displayName']()}</th>
						<th scope="col" class="px-4 py-3 font-medium">{m['common.status']()}</th>
						<th scope="col" class="px-4 py-3 font-medium">{m['admin.clusters.version']()}</th>
						<th scope="col" class="px-4 py-3 font-medium">{m['admin.clusters.nodesVms']()}</th>
						<th scope="col" class="px-4 py-3 font-medium">{m['admin.clusters.oidc']()}</th>
						<th scope="col" class="px-4 py-3 font-medium">{m['common.actions']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each store.clusters as cluster (cluster.name)}
						<tr class="border-t border-border align-top">
							<td class="px-4 py-3">
								<div class="font-medium">{cluster.name}</div>
								<div class="max-w-xs truncate text-xs text-muted-foreground" title={cluster.url}>{cluster.url}</div>
							</td>
							<td class="px-4 py-3 text-muted-foreground">{cluster.displayName || '—'}</td>
							<td class="px-4 py-3">
								<span class="rounded-full px-2 py-1 text-xs {statusClass(cluster.lastTestStatus)}">{statusLabel(cluster.lastTestStatus)}</span>
								{#if cluster.lastTestMessage}
									<div class="mt-1 text-xs text-muted-foreground">{cluster.lastTestMessage}</div>
								{:else if statusHint(cluster.lastTestStatus)}
									<div class="mt-1 text-xs text-muted-foreground">{statusHint(cluster.lastTestStatus)}</div>
								{/if}
							</td>
							<td class="px-4 py-3 text-muted-foreground">{cluster.proxmoxVersion ?? '—'}</td>
							<td class="px-4 py-3">{cluster.nodeCount} / {cluster.vmCount}</td>
							<td class="px-4 py-3">{cluster.oidcEnabled ? m['common.enabled']() : m['common.off']()}</td>
							<td class="px-4 py-3">
								<div class="flex flex-wrap gap-2">
									<Button variant="secondary" size="sm" disabled={store.busy !== null} label={m['admin.clusters.testLabel']({ name: cluster.name })} onclick={() => void store.test(cluster.name)}>{m['admin.clusters.test']()}</Button>
									<Button variant="secondary" size="sm" disabled={store.busy !== null} label={m['admin.clusters.editLabel']({ name: cluster.name })} onclick={() => editCluster(cluster)}>{m['common.edit']()}</Button>
									<Button variant="secondary" size="sm" disabled={store.busy !== null} label={cluster.oidcEnabled ? m['admin.clusters.disableOidcLabel']({ name: cluster.name }) : m['admin.clusters.enableOidcLabel']({ name: cluster.name })} onclick={() => void store.toggleOIDC(cluster.name, !cluster.oidcEnabled)}>{cluster.oidcEnabled ? m['admin.clusters.disableOidc']() : m['admin.clusters.enableOidc']()}</Button>
									<Button variant="destructive" size="sm" disabled={store.busy !== null} label={m['admin.clusters.removeLabel']({ name: cluster.name })} onclick={() => void store.remove(cluster.name)}>{m['admin.clusters.remove']()}</Button>
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>

<ClusterFormDialog
	bind:open={formOpen}
	{editing}
	saving={store.busy === 'create' || store.busy === `update:${editing?.name}`}
	error={store.error}
	onClose={() => (formOpen = false)}
	onSubmit={saveCluster}
/>
