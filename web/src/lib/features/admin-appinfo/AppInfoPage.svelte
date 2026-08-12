<script lang="ts">
	import { getAppInfoContext } from './appinfo.svelte';

	const store = getAppInfoContext();
</script>

<section class="space-y-6">
	<h1 class="text-2xl font-semibold tracking-tight">Application Info</h1>

	{#if store.loading}
		<p role="status" aria-live="polite" class="text-muted-foreground">Loading…</p>
	{:else if store.error}
		<p role="alert" class="text-destructive">{store.error}</p>
	{:else if store.info}
		<div role="status" aria-live="polite" class="sr-only">App info loaded</div>

		<div class="rounded-lg border border-border p-4">
			<p class="text-sm text-muted-foreground">Version</p>
			<p class="text-xl font-semibold">{store.info.version}</p>
		</div>

		<div class="space-y-2">
			<h2 class="text-lg font-medium">Configuration</h2>
			<div class="overflow-x-auto rounded-md border border-border">
				<table class="w-full text-sm">
					<thead class="bg-muted/50 text-left">
						<tr>
							<th class="px-4 py-2 font-medium">Field</th>
							<th class="px-4 py-2 font-medium">Value</th>
						</tr>
					</thead>
					<tbody>
						{#each store.info.config as field (field.name)}
							<tr class="border-t border-border">
								<td class="px-4 py-2 font-mono">{field.name}</td>
								<td class="px-4 py-2">
									{#if field.redacted}
										<span class="text-muted-foreground italic">redacted</span>
									{:else}
										<span class="font-mono">{field.value ?? ''}</span>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>

		<div class="space-y-2">
			<h2 class="text-lg font-medium">Cluster Health</h2>
			<div class="overflow-x-auto rounded-md border border-border">
				<table class="w-full text-sm">
					<thead class="bg-muted/50 text-left">
						<tr>
							<th class="px-4 py-2 font-medium">Cluster</th>
							<th class="px-4 py-2 font-medium">Last Refresh</th>
							<th class="px-4 py-2 font-medium">Status</th>
						</tr>
					</thead>
					<tbody>
						{#each store.info.clusters as cluster (cluster.name)}
							<tr class="border-t border-border">
								<td class="px-4 py-2">{cluster.name}</td>
								<td class="px-4 py-2 text-muted-foreground">
									{cluster.refreshedAt ? new Date(cluster.refreshedAt).toLocaleString() : 'never'}
								</td>
								<td class="px-4 py-2">
									{#if cluster.lastRefreshSucceeded}
										<span class="inline-flex items-center gap-1.5">
											<span class="h-2 w-2 rounded-full bg-green-500"></span>
											healthy
										</span>
									{:else}
										<span class="inline-flex items-center gap-1.5">
											<span class="h-2 w-2 rounded-full bg-red-500"></span>
											stale
										</span>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	{/if}
</section>
