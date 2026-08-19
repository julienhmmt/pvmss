<script lang="ts">
	import { getDashboardContext } from './dashboard.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';
	import StatusDot from '$lib/shared/ui/StatusDot.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getDashboardContext();

	function formatBytes(bytes: number): string {
		if (bytes === 0) return '0 B';
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		const i = Math.floor(Math.log(bytes) / Math.log(1024));
		return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
	}
</script>

<PageHeader title={m['admin.dashboard.title']()}>
	{#snippet actions()}
		<Button
			variant="secondary"
			size="sm"
			loading={store.loading}
			onclick={() => void store.load()}
		>
			{m['common.refresh']()}
		</Button>
	{/snippet}
</PageHeader>

{#if store.loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<div class="grid grid-cols-1 gap-4 sm:grid-cols-3" data-testid="dashboard-stats-skeleton">
		<Skeleton class="h-24 w-full" />
		<Skeleton class="h-24 w-full" />
		<Skeleton class="h-24 w-full" />
	</div>
	<div class="mt-6 space-y-2">
		<Skeleton class="h-6 w-40" />
		<TableSkeleton columns={2} />
	</div>
{:else if store.error}
	<p role="alert" class="text-destructive">{store.error}</p>
{:else if store.summary}
	<div role="status" aria-live="polite" class="sr-only">{m['admin.dashboard.loaded']()}</div>

	<section class="space-y-6">
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
			<div class="rounded-lg border border-border bg-card p-4">
				<p class="text-sm text-muted-foreground">{m['admin.dashboard.nodes']()}</p>
				<p class="text-3xl font-semibold">{store.summary.nodeCount}</p>
			</div>
			<div class="rounded-lg border border-border bg-card p-4">
				<p class="text-sm text-muted-foreground">{m['admin.dashboard.vms']()}</p>
				<p class="text-3xl font-semibold">{store.summary.vmCount}</p>
			</div>
			<div class="rounded-lg border border-border bg-card p-4">
				<p class="text-sm text-muted-foreground">{m['admin.dashboard.storageUsed']()}</p>
				<p class="text-3xl font-semibold">{formatBytes(store.summary.storageUsedBytes)}</p>
				<p class="text-sm text-muted-foreground">{m['admin.dashboard.storageOf']()} {formatBytes(store.summary.storageTotalBytes)}</p>
			</div>
		</div>

		<div class="space-y-2">
			<h2 class="text-lg font-medium">{m['admin.dashboard.nodesHeader']()}</h2>
			<div class="overflow-x-auto rounded-md border border-border">
				<table class="w-full text-sm">
					<thead class="bg-muted/50 text-left">
						<tr>
							<th class="px-4 py-2 font-medium">{m['common.name']()}</th>
							<th class="px-4 py-2 font-medium">{m['common.status']()}</th>
						</tr>
					</thead>
					<tbody>
						{#each store.summary.nodes as node (node.name)}
							<tr class="border-t border-border">
								<td class="px-4 py-2">{node.name}</td>
								<td class="px-4 py-2">
									<StatusDot
										tone={node.status === 'online' ? 'success' : 'destructive'}
										label={node.status}
									/>
								</td>
							</tr>
						{:else}
							<tr><td colspan={2} class="p-0"><EmptyState title={m['admin.dashboard.noNodes']()} /></td></tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>

		<div class="space-y-2">
			<h2 class="text-lg font-medium">{m['admin.dashboard.storageHeader']()}</h2>
			<div class="overflow-x-auto rounded-md border border-border">
				<table class="w-full text-sm">
					<thead class="bg-muted/50 text-left">
						<tr>
							<th class="px-4 py-2 font-medium">{m['common.name']()}</th>
							<th class="px-4 py-2 font-medium">{m['common.node']()}</th>
							<th class="px-4 py-2 font-medium">{m['common.type']()}</th>
							<th class="px-4 py-2 font-medium">{m['admin.dashboard.used']()}</th>
							<th class="px-4 py-2 font-medium">{m['common.total']()}</th>
						</tr>
					</thead>
					<tbody>
						{#each store.summary.storages as storage (storage.name + storage.node)}
							<tr class="border-t border-border">
								<td class="px-4 py-2">{storage.name}</td>
								<td class="px-4 py-2">{storage.node}</td>
								<td class="px-4 py-2">{storage.type}</td>
								<td class="px-4 py-2">{formatBytes(storage.usedBytes)}</td>
								<td class="px-4 py-2">{formatBytes(storage.totalBytes)}</td>
							</tr>
						{:else}
							<tr><td colspan={5} class="p-0"><EmptyState title={m['admin.dashboard.noStorages']()} /></td></tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>

		<p class="text-sm text-muted-foreground">
			{m['admin.dashboard.version']()} {store.summary.version} · {m['admin.dashboard.refreshed']()} {new Date(store.summary.refreshedAt).toLocaleString()}
		</p>
	</section>
{/if}
