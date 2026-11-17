<script lang="ts">
	import type { AdminTag } from './tags.svelte';
	import type { ClusterOption } from '#lib/shared/clusters.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import PageHeader from '#lib/shared/ui/PageHeader.svelte';
	import ClusterSelector from '#lib/shared/ui/ClusterSelector.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';
	import Select from '#lib/shared/ui/Select.svelte';
	import TableSkeleton from '#lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '#lib/shared/ui/EmptyState.svelte';
	import TagDrawer from './TagDrawer.svelte';
	import { m } from '#lib/paraglide/messages.js';

	type TagSortColumn = 'name' | 'vmCount';

	interface Props {
		tags: AdminTag[];
		filteredTags: AdminTag[];
		loading: boolean;
		error: string | null;
		saving: boolean;
		saveError: string | null;
		clusterOptions: ClusterOption[];
		cluster: string;
		onClusterChange: (value: string) => void;
		search: string;
		sortBy: TagSortColumn;
		sortDir: 'asc' | 'desc';
		onSearchChange: (value: string) => void;
		onSort: (column: TagSortColumn) => void;
		onResetFilters: () => void;
		onCreate: (name: string, color: string) => Promise<boolean>;
		onUpdateColor: (name: string, color: string) => Promise<boolean>;
		onDelete: (name: string) => Promise<boolean>;
		onClearSaveError: () => void;
	}

	let {
		tags,
		filteredTags,
		loading,
		error,
		saving,
		saveError,
		clusterOptions,
		cluster,
		onClusterChange,
		search,
		sortBy,
		sortDir,
		onSearchChange,
		onSort,
		onResetFilters,
		onCreate,
		onUpdateColor,
		onDelete,
		onClearSaveError
	}: Props = $props();

	let drawerTarget = $state<AdminTag | 'new' | null>(null);

	function openDrawer(target: AdminTag | 'new'): void {
		onClearSaveError();
		drawerTarget = target;
	}

	function closeDrawer(): void {
		drawerTarget = null;
		onClearSaveError();
	}

	const groups = $derived([
		{ id: 'system', title: m['admin.tags.groupSystem'](), items: filteredTags.filter((t) => t.protected) },
		{ id: 'custom', title: m['admin.tags.groupCustom'](), items: filteredTags.filter((t) => !t.protected) }
	]);

	const filtersActive = $derived(search !== '' || sortBy !== 'name' || sortDir !== 'asc');
</script>

<svelte:head>
	<title>{m['admin.tags.pageTitle']()}</title>
</svelte:head>

<PageHeader title={m['admin.tags.title']()} eyebrow={m['chrome.adminNav.provisioning']()}>
	{#snippet actions()}
		<ClusterSelector options={clusterOptions} value={cluster} onChange={onClusterChange} id="tags-cluster" />
		<Button onclick={() => openDrawer('new')}>{m['admin.tags.newTag']()}</Button>
	{/snippet}
</PageHeader>

{#if loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton columns={4} />
{:else if error}
	<Alert>{error}</Alert>
{:else if tags.length === 0}
	<EmptyState title={m['admin.tags.noTags']()}>
		{#snippet actions()}
			<Button onclick={() => openDrawer('new')}>{m['admin.tags.newTag']()}</Button>
		{/snippet}
	</EmptyState>
{:else}
	<div class="mb-6 flex flex-wrap items-center gap-3">
		<TextField
			type="search"
			placeholder={m['admin.tags.searchPlaceholder']()}
			aria-label={m['admin.tags.searchPlaceholder']()}
			value={search}
			oninput={(e: Event & { currentTarget: HTMLInputElement }) => onSearchChange(e.currentTarget.value)}
			class="w-full sm:w-56"
		/>
		<Select
			aria-label={m['admin.tags.sortLabel']()}
			value={sortBy}
			onchange={(e: Event & { currentTarget: HTMLSelectElement }) => onSort(e.currentTarget.value as TagSortColumn)}
			options={[
				{ value: 'name', label: m['common.name']() },
				{ value: 'vmCount', label: m['common.vms']() }
			]}
			class="w-full sm:w-36"
		/>
		<Button
			variant="secondary"
			size="sm"
			label={m['admin.tags.toggleSortDir']()}
			onclick={() => onSort(sortBy)}
		>{sortDir === 'asc' ? '↑' : '↓'}</Button>
		<span class="ml-auto text-sm text-muted-foreground" data-testid="tags-count">
			{m['admin.tags.resultCount']({ count: filteredTags.length })}
		</span>
		{#if filtersActive}
			<Button variant="secondary" size="sm" onclick={onResetFilters}>{m['admin.tags.resetFilters']()}</Button>
		{/if}
	</div>

	{#each groups as group (group.id)}
		<section class="mb-8" aria-labelledby="tags-group-{group.id}" data-testid="tags-group-{group.id}">
			<h2 id="tags-group-{group.id}" class="mb-3 text-xs font-medium uppercase tracking-wider text-muted-foreground">
				{group.title} <span class="ml-1">{group.items.length}</span>
			</h2>
			{#if group.items.length === 0}
				<p class="text-sm text-muted-foreground">{m['admin.tags.noFilterMatches']()}</p>
			{:else}
				<ul class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
					{#each group.items as tag (tag.name)}
						<li>
							<button
								type="button"
								class="pv-focus flex w-full items-center justify-between gap-3 rounded-xl border border-border bg-card p-4 text-left transition-colors hover:border-primary/60 hover:bg-muted/40"
								aria-label={m['admin.tags.openTagLabel']({ name: tag.name })}
								onclick={() => openDrawer(tag)}
							>
								<span class="min-w-0 space-y-2">
									<span class="block truncate rounded-full px-3 py-1 font-mono text-sm text-white" style="background: {tag.color}">{tag.name}</span>
									<span class="block font-mono text-xs text-muted-foreground">{tag.color}</span>
								</span>
								<span class="shrink-0 text-right">
									<span class="block text-lg font-medium tabular-nums">{tag.vmCount}</span>
									<span class="block text-xs text-muted-foreground">{m['common.vms']()}</span>
								</span>
							</button>
						</li>
					{/each}
				</ul>
			{/if}
		</section>
	{/each}
{/if}

<TagDrawer
	target={drawerTarget}
	{cluster}
	{saving}
	{saveError}
	{onCreate}
	{onUpdateColor}
	{onDelete}
	onClose={closeDrawer}
/>
