<script lang="ts">
	import type { AdminPool, CreatedPoolCredentials } from './pools.svelte';
	import type { ClusterOption } from '$lib/shared/clusters';
	import CreatePoolDialog from './CreatePoolDialog.svelte';
	import DeletePoolConfirm from './DeletePoolConfirm.svelte';
	import PoolVmBar from './PoolVmBar.svelte';
	import PoolCredentialsBanner from './PoolCredentialsBanner.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import TableHeader from '$lib/shared/ui/TableHeader.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import SearchIcon from '$lib/shared/ui/icons/SearchIcon.svelte';
	import TrashIcon from '$lib/shared/ui/icons/TrashIcon.svelte';
	import { m } from '$lib/paraglide/messages.js';

	type PoolSortColumn = 'name' | 'total' | 'running' | 'stopped';

	interface Props {
		pools: AdminPool[];
		loading: boolean;
		error: string | null;
		saving: boolean;
		saveError: string | null;
		deleting: string | null;
		deleteError: string | null;
		announce: string | null;
		credentials: CreatedPoolCredentials | null;
		clusterOptions: ClusterOption[];
		cluster: string;
		onClusterChange: (value: string) => void;
		onSearch: (value: string) => void;
		onCreate: (name: string, comment: string) => Promise<void>;
		onDelete: (name: string) => Promise<void>;
		onDismissCredentials: () => void;
	}

	let {
		pools,
		loading,
		error,
		saving,
		saveError,
		deleteError,
		deleting,
		announce,
		credentials,
		clusterOptions,
		cluster,
		onClusterChange,
		onSearch,
		onCreate,
		onDelete,
		onDismissCredentials
	}: Props = $props();
	let search = $state('');
	let showCreate = $state(false);
	let deleteName = $state<string | null>(null);
	let sortBy = $state<PoolSortColumn>('name');
	let sortDir = $state<'asc' | 'desc'>('asc');

	function openCreate(): void {
		showCreate = true;
	}

	function closeCreate(): void {
		showCreate = false;
	}

	function openDelete(name: string): void {
		deleteName = name;
	}

	function closeDelete(): void {
		deleteName = null;
	}

	async function confirmDelete(): Promise<void> {
		if (deleteName) {
			try {
				await onDelete(deleteName);
				deleteName = null;
			} catch {
				// error is set on the store; dialog stays open
			}
		}
	}

	function handleSort(column: string): void {
		const next = column as PoolSortColumn;
		if (sortBy === next) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortBy = next;
			sortDir = 'asc';
		}
	}

	function sortPools(list: AdminPool[], by: PoolSortColumn, dir: 'asc' | 'desc'): AdminPool[] {
		const sorted = [...list];
		sorted.sort((a, b) => {
			let cmp = 0;
			switch (by) {
				case 'name':
					cmp = a.name.localeCompare(b.name);
					break;
				case 'total':
					cmp = a.total - b.total;
					break;
				case 'running':
					cmp = a.running - b.running;
					break;
				case 'stopped':
					cmp = a.stopped - b.stopped;
					break;
			}
			if (cmp !== 0) return cmp;
			return a.name.localeCompare(b.name);
		});
		return dir === 'desc' ? sorted.reverse() : sorted;
	}

	function resetFilters(): void {
		search = '';
		onSearch('');
		sortBy = 'name';
		sortDir = 'asc';
	}

	const filteredPools = $derived(
		sortPools(
			pools.filter((pool) => pool.managed),
			sortBy,
			sortDir
		)
	);

	const resultCountText = $derived(
		filteredPools.length === 1
			? m['admin.pools.resultCountSingular']({ count: 1 })
			: m['admin.pools.resultCount']({ count: filteredPools.length })
	);

	const hasFilters = $derived(search !== '' || sortBy !== 'name' || sortDir !== 'asc');

	function emptyTitle(): string {
		return search !== '' ? m['admin.pools.noSearchResults']() : m['admin.pools.noPools']();
	}

	function updateSearch(event: Event & { currentTarget: HTMLInputElement }): void {
		const value = event.currentTarget.value;
		search = value;
		onSearch(value);
	}
</script>

<svelte:head>
	<title>{m['admin.pools.pageTitle']()}</title>
</svelte:head>

<div class="sr-only" role="status" aria-live="polite">{announce ?? ''}</div>

<PageHeader title={m['admin.pools.heading']()} description={m['admin.pools.description']()}>
	{#snippet actions()}
		<ClusterSelector options={clusterOptions} value={cluster} onChange={onClusterChange} id="pools-cluster" />
		<Button onclick={openCreate}>{m['admin.pools.newPool']()}</Button>
	{/snippet}
</PageHeader>

{#if credentials}
	<PoolCredentialsBanner {credentials} onDismiss={onDismissCredentials} />
{/if}

{#if loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton columns={7} />
{:else if error}
	<p role="alert" class="text-destructive">{error}</p>
{:else}
	{#if saveError}
		<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">{saveError}</p>
	{/if}
	{#if deleteError}
		<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">{deleteError}</p>
	{/if}

	<div class="overflow-hidden rounded-lg border border-border bg-card shadow-card">
		<div class="flex flex-wrap items-center gap-3 border-b border-border px-4 py-3">
			<div class="relative w-full max-w-xs">
				<span class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-muted-foreground">
					<SearchIcon class="h-4 w-4" />
				</span>
				<input
					type="search"
					class="pv-input pl-9"
					placeholder={m['admin.pools.searchPlaceholder']()}
					aria-label={m['admin.pools.search']()}
					bind:value={search}
					oninput={updateSearch}
				/>
			</div>
			{#if hasFilters}
				<Button variant="ghost" size="sm" onclick={resetFilters}>{m['admin.pools.resetFilters']()}</Button>
			{/if}
			<span class="ml-auto text-sm text-muted-foreground">{resultCountText}</span>
		</div>

		<div class="overflow-x-auto">
			<table class="pv-responsive-table w-full text-sm">
				<caption class="sr-only">{m['admin.pools.heading']()}</caption>
				<thead class="bg-muted/60 text-left text-sm font-medium text-muted-foreground">
					<tr>
						<TableHeader text={m['common.name']()} column="name" activeColumn={sortBy} {sortDir} onSort={handleSort} />
						<th class="px-4 py-3 font-medium">{m['admin.pools.comment']()}</th>
						<TableHeader text={m['admin.pools.vmsColumn']()} tooltip={m['admin.pools.vmsTooltip']()} class="text-center" />
						<TableHeader text={m['common.total']()} column="total" activeColumn={sortBy} {sortDir} onSort={handleSort} class="text-center" />
						<TableHeader text={m['common.running']()} column="running" activeColumn={sortBy} {sortDir} onSort={handleSort} class="text-center" />
						<TableHeader text={m['common.stopped']()} column="stopped" activeColumn={sortBy} {sortDir} onSort={handleSort} class="text-center" />
						<th class="px-4 py-3 text-right font-medium">{m['common.actions']()}</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-border">
					{#each filteredPools as pool (pool.name)}
						<tr class="group transition-colors hover:bg-muted/40">
							<td class="px-4 py-3.5 font-mono" data-label={m['common.name']()}>{pool.name}</td>
							<td class="px-4 py-3.5 text-muted-foreground" data-label={m['admin.pools.comment']()}>
								{#if pool.comment}
									<span class="block max-w-xs truncate" title={pool.comment}>{pool.comment}</span>
								{:else}
									<span class="text-muted-foreground-subtle">—</span>
								{/if}
							</td>
							<td class="px-4 py-3.5 text-center" data-label={m['admin.pools.vmsColumn']()}>
								<PoolVmBar running={pool.running} stopped={pool.stopped} total={pool.total} />
							</td>
							<td class="px-4 py-3.5 text-center font-mono tabular-nums" data-label={m['common.total']()}>{pool.total}</td>
							<td class="px-4 py-3.5 text-center font-mono tabular-nums" data-label={m['common.running']()}>{pool.running}</td>
							<td class="px-4 py-3.5 text-center font-mono tabular-nums" data-label={m['common.stopped']()}>{pool.stopped}</td>
							<td class="px-4 py-3.5 text-right" data-label={m['common.actions']()} data-nolabel="true">
								<Button
									variant="ghost"
									size="sm"
									label={m['admin.pools.deletePoolLabel']({ name: pool.name })}
									onclick={() => openDelete(pool.name)}
								>
									<TrashIcon class="h-4 w-4 text-destructive" />
								</Button>
							</td>
						</tr>
					{:else}
						<tr>
							<td colspan={7} class="p-0">
								{#if filteredPools.length === 0 && search === ''}
									<EmptyState title={emptyTitle()}>
										{#snippet actions()}
											<Button onclick={openCreate}>{m['admin.pools.newPool']()}</Button>
										{/snippet}
									</EmptyState>
								{:else}
									<EmptyState title={emptyTitle()} />
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>
{/if}

<CreatePoolDialog
	bind:open={showCreate}
	saving={saving}
	error={saveError}
	onClose={closeCreate}
	onCreate={async (name, comment) => {
		try {
			await onCreate(name, comment);
			closeCreate();
		} catch {
			/* error shown via saveError */
		}
	}}
/>
{#if deleteName}
	<DeletePoolConfirm
		open={true}
		poolName={deleteName}
		deleting={deleting === deleteName}
		error={deleteError}
		onClose={closeDelete}
		onConfirm={confirmDelete}
	/>
{/if}
