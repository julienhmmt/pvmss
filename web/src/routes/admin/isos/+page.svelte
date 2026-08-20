<script lang="ts">
	import { onMount } from 'svelte';
	import { setAdminCatalogContext } from '$lib/features/admin-catalog/admin-catalog.svelte';
	import IsosTable from '$lib/features/admin-catalog/IsosTable.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = setAdminCatalogContext();

	onMount(() => {
		void store.loadAll();
	});
</script>

<svelte:head>
	<title>{m['admin.isos.title']()}</title>
</svelte:head>

<PageHeader title={m['admin.isos.heading']()}>
	{#snippet actions()}
		<ClusterSelector options={store.clusterOptions} value={store.cluster} onChange={(value) => store.setCluster(value)} id="isos-cluster" />
	{/snippet}
</PageHeader>

{#if store.loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton columns={5} />
{:else if store.error}
	<p role="alert" class="text-destructive">{store.error}</p>
{:else}
	<div role="status" aria-live="polite" class="sr-only">{m['admin.isos.isosLoaded']({ count: store.isos.length })}</div>

	{#if store.toggleError}
		<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">
			{store.toggleError}
		</p>
	{/if}

	<div class="mb-4 flex flex-wrap items-center gap-2">
		<input
			type="search"
			class="rounded-md border border-border bg-background px-3 py-1.5 text-sm"
			placeholder={m['admin.isos.searchPlaceholder']()}
			bind:value={store.isoSearch}
		/>
		<select class="rounded-md border border-border bg-background px-3 py-1.5 text-sm" bind:value={store.isoStorageFilter}>
			<option value="">{m['admin.isos.filterStorage']()}</option>
			{#each store.isoStorageOptions as storage}
				<option value={storage}>{storage}</option>
			{/each}
		</select>
		<select class="rounded-md border border-border bg-background px-3 py-1.5 text-sm" bind:value={store.isoNodeFilter}>
			<option value="">{m['admin.isos.filterNode']()}</option>
			{#each store.isoNodeOptions as node}
				<option value={node}>{node}</option>
			{/each}
		</select>
		<select class="rounded-md border border-border bg-background px-3 py-1.5 text-sm" bind:value={store.isoEnabledFilter}>
			<option value="all">{m['admin.isos.filterEnabled']()}</option>
			<option value="enabled">{m['admin.isos.filterEnabledOnly']()}</option>
			<option value="disabled">{m['admin.isos.filterDisabledOnly']()}</option>
		</select>
		<button
			class="rounded-md border border-border px-3 py-1.5 text-sm text-muted-foreground hover:bg-muted"
			onclick={() => store.resetISOFilters()}
		>
			{m['admin.isos.resetFilters']()}
		</button>
	</div>

	<div class="fade-in">
		{#if store.filteredIsos.length === 0 && store.isos.length > 0}
			<EmptyState title={m['admin.isos.noFilterMatches']()} />
		{:else}
			<IsosTable
				isos={store.filteredIsos}
				toggling={store.toggling}
				onToggle={(node, storage, file, enabled) => void store.toggleISO(node, storage, file, enabled)}
				sortBy={store.isoSortBy}
				sortDir={store.isoSortDir}
				onSort={(column: 'file' | 'storage' | 'node' | 'size' | 'enabled') => store.setISOSort(column)}
			/>
		{/if}
	</div>
{/if}
