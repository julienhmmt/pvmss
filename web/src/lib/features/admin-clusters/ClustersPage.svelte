<script lang="ts">
	import ClusterFormDialog from './ClusterFormDialog.svelte';
	import type { AdminCluster, AdminClustersStore, ClusterInput } from './clusters.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';

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

	function saveCluster(input: ClusterInput): void {
		formOpen = false;
		if (editing === null) {
			void store.create(input);
			return;
		}
		const update: Omit<ClusterInput, 'name'> = {
			url: input.url,
			tlsInsecureSkipVerify: input.tlsInsecureSkipVerify,
			tokenId: input.tokenId,
			tokenSecret: input.tokenSecret
		};
		void store.update(editing.name, update);
	}
</script>

<PageHeader title="Clusters" description="Manage connection targets and per-cluster login options.">
	{#snippet actions()}
		<Button onclick={addCluster}>Add cluster</Button>
	{/snippet}
</PageHeader>

<section class="mx-auto w-full max-w-6xl">
	{#if store.announce}<p class="sr-only" role="status" aria-live="polite">{store.announce}</p>{/if}
	{#if store.error}<p class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive" role="alert">{store.error}</p>{/if}
	{#if store.loading}
		<p role="status" aria-live="polite" class="text-muted-foreground">Loading clusters…</p>
	{:else}
		<div class="overflow-x-auto rounded-lg border border-border">
			<table class="w-full min-w-[900px] text-left text-sm">
				<caption class="sr-only">Configured Proxmox clusters</caption>
				<thead class="bg-muted/50">
					<tr>
						<th scope="col" class="px-4 py-3 font-medium">Name</th>
						<th scope="col" class="px-4 py-3 font-medium">Status</th>
						<th scope="col" class="px-4 py-3 font-medium">Version</th>
						<th scope="col" class="px-4 py-3 font-medium">Nodes / VMs</th>
						<th scope="col" class="px-4 py-3 font-medium">OIDC</th>
						<th scope="col" class="px-4 py-3 font-medium">Actions</th>
					</tr>
				</thead>
				<tbody>
					{#each store.clusters as cluster (cluster.name)}
						<tr class="border-t border-border align-top">
							<td class="px-4 py-3">
								<div class="font-medium">{cluster.name}</div>
								<div class="max-w-xs truncate text-xs text-muted-foreground" title={cluster.url}>{cluster.url}</div>
							</td>
							<td class="px-4 py-3">
								<span class="rounded-full bg-muted px-2 py-1 text-xs">{cluster.lastTestStatus ?? 'untested'}</span>
								{#if cluster.lastTestMessage}<div class="mt-1 text-xs text-muted-foreground">{cluster.lastTestMessage}</div>{/if}
							</td>
							<td class="px-4 py-3 text-muted-foreground">{cluster.proxmoxVersion ?? '—'}</td>
							<td class="px-4 py-3">{cluster.nodeCount} / {cluster.vmCount}</td>
							<td class="px-4 py-3">{cluster.oidcEnabled ? 'Enabled' : 'Off'}</td>
							<td class="px-4 py-3">
								<div class="flex flex-wrap gap-2">
									<button type="button" class="rounded-md border border-border px-2 py-1 text-xs" disabled={store.busy !== null} onclick={() => void store.test(cluster.name)}>Test</button>
									<button type="button" class="rounded-md border border-border px-2 py-1 text-xs" disabled={store.busy !== null} onclick={() => editCluster(cluster)}>Edit</button>
									<button type="button" class="rounded-md border border-border px-2 py-1 text-xs" disabled={store.busy !== null} onclick={() => void store.toggleOIDC(cluster.name, !cluster.oidcEnabled)}>{cluster.oidcEnabled ? 'Disable OIDC' : 'Enable OIDC'}</button>
									<button type="button" class="rounded-md border border-destructive px-2 py-1 text-xs text-destructive" disabled={store.busy !== null} onclick={() => void store.remove(cluster.name)}>Remove</button>
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>

<ClusterFormDialog bind:open={formOpen} {editing} onClose={() => (formOpen = false)} onSubmit={saveCluster} />
