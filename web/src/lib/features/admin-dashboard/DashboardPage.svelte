<script lang="ts">
	import { getDashboardContext } from './dashboard.svelte';

	const store = getDashboardContext();

	function formatBytes(bytes: number): string {
		if (bytes === 0) return '0 B';
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		const i = Math.floor(Math.log(bytes) / Math.log(1024));
		return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
	}
</script>

<section class="space-y-6">
	<h1 class="text-2xl font-semibold tracking-tight">Dashboard</h1>

	{#if store.loading}
		<p role="status" aria-live="polite" class="text-muted-foreground">Loading…</p>
	{:else if store.error}
		<p role="alert" class="text-destructive">{store.error}</p>
	{:else if store.summary}
		<div role="status" aria-live="polite" class="sr-only">Dashboard loaded</div>

		<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
			<div class="rounded-lg border border-border bg-background p-4">
				<p class="text-sm text-muted-foreground">Nodes</p>
				<p class="text-3xl font-semibold">{store.summary.nodeCount}</p>
			</div>
			<div class="rounded-lg border border-border bg-background p-4">
				<p class="text-sm text-muted-foreground">VMs</p>
				<p class="text-3xl font-semibold">{store.summary.vmCount}</p>
			</div>
			<div class="rounded-lg border border-border bg-background p-4">
				<p class="text-sm text-muted-foreground">Storage Used</p>
				<p class="text-3xl font-semibold">{formatBytes(store.summary.storageUsedBytes)}</p>
				<p class="text-sm text-muted-foreground">of {formatBytes(store.summary.storageTotalBytes)}</p>
			</div>
		</div>

		<div class="space-y-2">
			<h2 class="text-lg font-medium">Nodes</h2>
			<div class="overflow-x-auto rounded-md border border-border">
				<table class="w-full text-sm">
					<thead class="bg-muted/50 text-left">
						<tr>
							<th class="px-4 py-2 font-medium">Name</th>
							<th class="px-4 py-2 font-medium">Status</th>
						</tr>
					</thead>
					<tbody>
						{#each store.summary.nodes as node (node.name)}
							<tr class="border-t border-border">
								<td class="px-4 py-2">{node.name}</td>
								<td class="px-4 py-2">
									<span class="inline-flex items-center gap-1.5">
										<span class="h-2 w-2 rounded-full {node.status === 'online' ? 'bg-green-500' : 'bg-red-500'}"></span>
										{node.status}
									</span>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>

		<div class="space-y-2">
			<h2 class="text-lg font-medium">Storage</h2>
			<div class="overflow-x-auto rounded-md border border-border">
				<table class="w-full text-sm">
					<thead class="bg-muted/50 text-left">
						<tr>
							<th class="px-4 py-2 font-medium">Name</th>
							<th class="px-4 py-2 font-medium">Node</th>
							<th class="px-4 py-2 font-medium">Type</th>
							<th class="px-4 py-2 font-medium">Used</th>
							<th class="px-4 py-2 font-medium">Total</th>
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
						{/each}
					</tbody>
				</table>
			</div>
		</div>

		<p class="text-sm text-muted-foreground">
			Version {store.summary.version} · Refreshed {new Date(store.summary.refreshedAt).toLocaleString()}
		</p>
	{/if}
</section>
