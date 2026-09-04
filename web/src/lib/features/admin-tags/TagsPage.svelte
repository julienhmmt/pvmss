<script lang="ts">
	import type { AdminTag } from './tags.svelte';
	import type { ClusterOption } from '$lib/shared/clusters';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import TableCard from '$lib/shared/ui/TableCard.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import ConfirmDialog from '$lib/shared/ui/ConfirmDialog.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import ColorPicker from '$lib/shared/ui/ColorPicker.svelte';
	import TableHeader from '$lib/shared/ui/TableHeader.svelte';
	import { m } from '$lib/paraglide/messages.js';

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
		protectedFilter: 'all' | 'protected' | 'unprotected';
		sortBy: TagSortColumn;
		sortDir: 'asc' | 'desc';
		onSearchChange: (value: string) => void;
		onProtectedFilterChange: (value: 'all' | 'protected' | 'unprotected') => void;
		onSort: (column: TagSortColumn) => void;
		onResetFilters: () => void;
		onCreate: (name: string, color: string) => void;
		onUpdateColor: (name: string, color: string) => void;
		onDelete: (name: string) => void;
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
		protectedFilter,
		sortBy,
		sortDir,
		onSearchChange,
		onProtectedFilterChange,
		onSort,
		onResetFilters,
		onCreate,
		onUpdateColor,
		onDelete
	}: Props = $props();

	function handleSort(column: string): void {
		onSort(column as TagSortColumn);
	}

	let showForm = $state(false);
	let newName = $state('');
	let newColor = $state('#4f46e5');
	let editingTag = $state<string | null>(null);
	let editColor = $state('');
	let pendingDelete = $state<AdminTag | null>(null);

	function openCreate(): void {
		newName = '';
		newColor = '#4f46e5';
		showForm = true;
	}

	function submitCreate(): void {
		onCreate(newName, newColor);
		showForm = false;
	}

	function startEdit(tag: AdminTag): void {
		editingTag = tag.name;
		editColor = tag.color;
	}

	function saveEdit(): void {
		if (editingTag) {
			onUpdateColor(editingTag, editColor);
		}
		editingTag = null;
	}
</script>

<svelte:head>
	<title>{m['admin.tags.pageTitle']()}</title>
</svelte:head>

<PageHeader title={m['admin.tags.title']()}>
	{#snippet actions()}
		<ClusterSelector options={clusterOptions} value={cluster} onChange={onClusterChange} id="tags-cluster" />
		<Button onclick={openCreate}>{m['admin.tags.newTag']()}</Button>
	{/snippet}
</PageHeader>

{#if loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton columns={4} />
{:else if error}
	<p role="alert" class="text-destructive">{error}</p>
{:else}
	{#if saveError}
		<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">
			{saveError}
		</p>
	{/if}

	{#if tags.length > 0}
		<TableCard>
			{#snippet toolbar()}
				<TextField
					type="search"
					placeholder={m['admin.tags.searchPlaceholder']()}
					value={search}
					oninput={(e: Event & { currentTarget: HTMLInputElement }) => onSearchChange(e.currentTarget.value)}
					class="w-full sm:w-48"
				/>
				<Select
					value={protectedFilter}
					onchange={(e: Event & { currentTarget: HTMLSelectElement }) => onProtectedFilterChange(e.currentTarget.value as 'all' | 'protected' | 'unprotected')}
					options={[
						{ value: 'all', label: m['admin.tags.filterProtected']() },
						{ value: 'protected', label: m['admin.tags.filterProtectedOnly']() },
						{ value: 'unprotected', label: m['admin.tags.filterUnprotectedOnly']() }
					]}
					class="w-full sm:w-44"
				/>
				<Button
					variant="secondary"
					size="sm"
					onclick={onResetFilters}
				>
					{m['admin.tags.resetFilters']()}
				</Button>
			{/snippet}
			<table class="pv-table pv-responsive-table">
				<caption class="sr-only">{m['admin.tags.title']()}</caption>
				<thead>
					<tr>
						<TableHeader text={m['common.name']()} tooltip={m['admin.tags.tooltip.color']()} column="name" activeColumn={sortBy} {sortDir} onSort={handleSort} />
						<TableHeader text={m['admin.tags.color']()} tooltip={m['admin.tags.tooltip.color']()} />
						<TableHeader text={m['common.vms']()} tooltip={m['admin.tags.tooltip.vmCount']()} column="vmCount" activeColumn={sortBy} {sortDir} onSort={handleSort} />
						<th class="font-medium">{m['common.actions']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each filteredTags as tag (tag.name)}
						<tr class="group transition-colors hover:bg-muted/40">
							<td class="font-mono" data-label={m['common.name']()}>
								{tag.name}
								{#if tag.protected}
									<span class="ml-2 rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">{m['admin.tags.protected']()}</span>
								{/if}
							</td>
							<td data-label={m['admin.tags.color']()}>
								{#if editingTag === tag.name}
									<div class="flex items-center gap-2">
										<ColorPicker bind:value={editColor} class="h-8 w-12 p-1" />
										<Button variant="ghost" size="sm" onclick={saveEdit}>{m['common.save']()}</Button>
										<Button variant="ghost" size="sm" onclick={() => (editingTag = null)}>{m['common.cancel']()}</Button>
									</div>
								{:else}
									<div class="flex items-center gap-2">
										<span class="inline-block h-4 w-4 rounded-full border" style="background: {tag.color}"></span>
										<span class="font-mono text-xs">{tag.color}</span>
										<Button variant="ghost" size="sm" label={m['admin.tags.editColorLabel']({ name: tag.name })} onclick={() => startEdit(tag)}>{m['common.edit']()}</Button>
									</div>
								{/if}
							</td>
							<td data-label={m['common.vms']()}>{tag.vmCount}</td>
							<td data-label={m['common.actions']()}>
								{#if !tag.protected}
									<Button variant="destructive" size="sm" label={m['admin.tags.deleteLabel']({ name: tag.name })} onclick={() => (pendingDelete = tag)}>{m['common.delete']()}</Button>
								{:else}
									<span class="text-xs text-muted-foreground">—</span>
								{/if}
							</td>
						</tr>
					{:else}
						<tr><td colspan={4} class="p-0">
							<EmptyState title={m['admin.tags.noFilterMatches']()} />
						</td></tr>
					{/each}
				</tbody>
			</table>
		</TableCard>
	{:else}
		<EmptyState title={m['admin.tags.noTags']()}>
			{#snippet actions()}
				<Button onclick={openCreate}>{m['admin.tags.newTag']()}</Button>
			{/snippet}
		</EmptyState>
	{/if}
{/if}

<Dialog bind:open={showForm} size="sm" labelledBy="tag-form-title" onClose={() => (showForm = false)}>
	<h2 id="tag-form-title" class="mb-4 text-lg font-medium">{m['admin.tags.newTagForm']()}</h2>
	<form onsubmit={(e) => { e.preventDefault(); submitCreate(); }} class="space-y-4">
		<FormField label={m['admin.tags.nameField']()} required>
			{#snippet children({ id, describedBy, invalid })}
				<TextField
					{id}
					{describedBy}
					{invalid}
					type="text"
					pattern={'[a-zA-Z0-9]{1,50}'}
					bind:value={newName}
					required
				/>
			{/snippet}
		</FormField>
		<FormField label={m['admin.tags.colorField']()} required>
			{#snippet children({ id, describedBy, invalid })}
				<ColorPicker {id} {describedBy} {invalid} bind:value={newColor} class="h-10 w-full" required />
			{/snippet}
		</FormField>
		<div class="flex justify-end gap-2 pt-2">
			<Button variant="ghost" onclick={() => (showForm = false)}>{m['common.cancel']()}</Button>
			<Button type="submit" disabled={saving}>
				{saving ? m['common.creating']() : m['common.create']()}
			</Button>
		</div>
	</form>
</Dialog>

<ConfirmDialog
	open={pendingDelete !== null}
	title={m['admin.tags.deleteTitle']({ name: pendingDelete?.name ?? '' })}
	message={m['admin.tags.deleteConfirm']()}
	confirmLabel={m['common.deletePermanently']()}
	cancelLabel={m['common.cancel']()}
	confirming={saving}
	testId="tag-delete-confirm"
	onConfirm={() => { if (pendingDelete) { onDelete(pendingDelete.name); pendingDelete = null; } }}
	onClose={() => (pendingDelete = null)}
/>
