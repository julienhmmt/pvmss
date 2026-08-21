<script lang="ts">
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';
	import { getSearchContext } from './search.svelte';
	import type { VmListItem, VmStatus } from '$lib/features/vms/list.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';

	const store = getSearchContext();

	const statusClasses: Record<VmStatus, string> = {
		running: 'bg-success-soft text-success-soft-foreground',
		stopped: 'bg-muted text-muted-foreground',
		paused: 'bg-destructive-soft text-destructive-soft-foreground'
	};

	const statusLabels: Record<VmStatus, () => string> = {
		running: () => m['common.statusRunning'](),
		stopped: () => m['common.statusStopped'](),
		paused: () => m['common.statusPaused']()
	};

	function vmHref(machine: VmListItem): string {
		return resolve(`/vms/${encodeURIComponent(machine.cluster)}/${machine.vmid}`);
	}

	function handleInput(event: Event): void {
		store.applySearch((event.currentTarget as HTMLInputElement).value);
	}
</script>

<section class="mx-auto w-full max-w-5xl px-4 py-8">
	<h1 class="mb-6 text-2xl font-semibold tracking-tight">{m['search.heading']()}</h1>

	<div class="mb-6">
		<label for="global-search" class="sr-only">{m['search.label']()}</label>
		<input
			id="global-search"
			type="search"
			placeholder={m['search.placeholder']()}
			class="w-full max-w-md rounded-md border border-border bg-background px-3 py-1.5 text-sm"
			value={store.query}
			oninput={handleInput}
			data-testid="global-search-input"
		/>
	</div>

	{#if store.loading && store.result === null}
		<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
		<TableSkeleton columns={5} />
	{:else if store.error}
		<p role="alert" class="text-sm text-destructive" data-testid="search-error">{store.error}</p>
	{:else if store.result === null}
		<EmptyState title={m['search.instructions']()} dataTestid="search-instructions" />
	{:else if store.result.items.length === 0}
		<EmptyState title={m['search.empty']()} dataTestid="search-empty" />
	{:else}
		<table class="pv-responsive-table text-sm">
			<caption class="sr-only">{m['search.resultsCaption']()}</caption>
			<thead>
				<tr class="border-b border-border">
					<th scope="col" class="px-3 py-2 font-medium">{m['vms.list.columnCluster']()}</th>
					<th scope="col" class="px-3 py-2 font-medium">{m['vms.list.columnId']()}</th>
					<th scope="col" class="px-3 py-2 font-medium">{m['vms.list.columnName']()}</th>
					<th scope="col" class="px-3 py-2 font-medium">{m['vms.list.columnNode']()}</th>
					<th scope="col" class="px-3 py-2 font-medium">{m['vms.list.columnStatus']()}</th>
				</tr>
			</thead>
			<tbody>
				{#each store.result.items as machine (`${machine.cluster}:${machine.vmid}`)}
					<tr class="border-b border-border last:border-0" data-testid="search-result-row">
						<td class="px-3 py-2 font-mono text-muted-foreground" data-label={m['vms.list.columnCluster']()}>
							{machine.clusterDisplayName}
						</td>
						<td class="px-3 py-2 font-mono text-muted-foreground" data-label={m['vms.list.columnId']()}>
							{machine.vmid}
						</td>
						<td class="px-3 py-2" data-label={m['vms.list.columnName']()}>
							<a
								href={vmHref(machine)}
								class="font-medium hover:underline"
								data-testid="search-result-link"
							>
								{machine.name}
							</a>
							{#if machine.tags.length > 0}
								<div class="mt-1 flex flex-wrap gap-1">
									{#each machine.tags as tag (tag)}
										<span
											class="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground"
											data-testid="search-result-tag"
										>
											{tag}
										</span>
									{/each}
								</div>
							{/if}
						</td>
						<td class="px-3 py-2 font-mono text-muted-foreground" data-label={m['vms.list.columnNode']()}>
							{machine.node}
						</td>
						<td class="px-3 py-2" data-label={m['vms.list.columnStatus']()}>
							<span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs {statusClasses[machine.status]}">
								{statusLabels[machine.status]()}
							</span>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</section>
