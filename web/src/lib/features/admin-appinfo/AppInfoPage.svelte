<script lang="ts">
	import { getAppInfoContext } from './appinfo.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import StatusDot from '$lib/shared/ui/StatusDot.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getAppInfoContext();
</script>

<PageHeader title={m['admin.appinfo.heading']()} />

<section class="space-y-6">
	{#if store.loading}
		<p role="status" aria-live="polite" class="text-muted-foreground">{m['common.loading']()}</p>
	{:else if store.error}
		<p role="alert" class="text-destructive">{store.error}</p>
	{:else if store.info}
		<div role="status" aria-live="polite" class="sr-only">{m['admin.appinfo.loaded']()}</div>

		<div class="rounded-lg border border-border p-4">
			<p class="text-sm text-muted-foreground">{m['admin.appinfo.version']()}</p>
			<p class="text-xl font-semibold">{store.info.version}</p>
		</div>

		<div class="space-y-2">
			<h2 class="text-lg font-medium">{m['admin.appinfo.configuration']()}</h2>
			<div class="overflow-x-auto rounded-md border border-border">
				<table class="w-full text-sm">
					<thead class="bg-muted/50 text-left">
						<tr>
							<th class="px-4 py-2 font-medium">{m['admin.appinfo.field']()}</th>
							<th class="px-4 py-2 font-medium">{m['admin.appinfo.value']()}</th>
						</tr>
					</thead>
					<tbody>
						{#each store.info.config as field (field.name)}
							<tr class="border-t border-border">
								<td class="px-4 py-2 font-mono">{field.name}</td>
								<td class="px-4 py-2">
									{#if field.redacted}
										<span class="text-muted-foreground italic">{m['admin.appinfo.redacted']()}</span>
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
			<h2 class="text-lg font-medium">{m['admin.appinfo.clusterHealth']()}</h2>
			<div class="overflow-x-auto rounded-md border border-border">
				<table class="w-full text-sm">
					<thead class="bg-muted/50 text-left">
						<tr>
							<th class="px-4 py-2 font-medium">{m['common.cluster']()}</th>
							<th class="px-4 py-2 font-medium">{m['admin.appinfo.lastRefresh']()}</th>
							<th class="px-4 py-2 font-medium">{m['common.status']()}</th>
						</tr>
					</thead>
					<tbody>
						{#each store.info.clusters as cluster (cluster.name)}
							<tr class="border-t border-border">
								<td class="px-4 py-2">{cluster.name}</td>
								<td class="px-4 py-2 text-muted-foreground">
									{cluster.refreshedAt ? new Date(cluster.refreshedAt).toLocaleString() : m['admin.appinfo.never']()}
								</td>
								<td class="px-4 py-2">
									<StatusDot
										tone={cluster.lastRefreshSucceeded ? 'success' : 'destructive'}
										label={cluster.lastRefreshSucceeded ? m['admin.appinfo.healthy']() : m['admin.appinfo.stale']()}
									/>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	{/if}
</section>
