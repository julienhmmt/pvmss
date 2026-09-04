<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import {
		setAdminCatalogContext,
		type AdminTemplate,
		type AdminTemplatePatch,
		type TemplateSortColumn
	} from '$lib/features/admin-catalog/admin-catalog.svelte';
	import TemplatesTable from '$lib/features/admin-catalog/TemplatesTable.svelte';
	import TemplatesTableToolbar from '$lib/features/admin-catalog/TemplatesTableToolbar.svelte';
	import TemplateEditDialog from '$lib/features/admin-catalog/TemplateEditDialog.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import TableCard from '$lib/shared/ui/TableCard.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = setAdminCatalogContext();
	const toast = getToastContext();

	let editing = $state<AdminTemplate | null>(null);

	onMount(() => {
		void store.loadTemplates();
	});

	function handleToggle(vmid: number, enabled: boolean): void {
		void performToggle(vmid, enabled);
	}

	function handleRemove(vmid: number): void {
		void performRemove(vmid);
	}

	function handleEdit(template: AdminTemplate): void {
		editing = template;
	}

	async function handleEditSave(patch: AdminTemplatePatch): Promise<void> {
		if (editing === null) return;
		const vmid = editing.vmid;
		try {
			await store.updateTemplate(vmid, patch);
			editing = null;
			toast.success(m['admin.templates.updateSuccess']({ vmid }));
		} catch {
			toast.error(m['admin.templates.updateError']());
		}
	}

	async function performToggle(vmid: number, enabled: boolean): Promise<void> {
		try {
			await store.toggleTemplate(vmid, enabled);
			toast.success(
				enabled ? m['admin.templates.enabledSuccess']({ vmid }) : m['admin.templates.disabledSuccess']({ vmid })
			);
		} catch {
			toast.error(m['admin.catalog.toggleTemplateError']());
		}
	}

	async function performRemove(vmid: number): Promise<void> {
		try {
			await store.removeTemplate(vmid);
			toast.success(m['admin.templates.removeSuccess']({ vmid }));
		} catch {
			toast.error(m['admin.templates.removeError']());
		}
	}

	function handleSort(column: TemplateSortColumn): void {
		store.setTemplateSort(column);
	}

	function handleClusterChange(value: string): void {
		store.cluster = value;
		void store.loadTemplates();
	}
</script>

<svelte:head>
	<title>{m['admin.templates.title']()}</title>
</svelte:head>

<PageHeader title={m['admin.templates.heading']()}>
	{#snippet actions()}
		<ClusterSelector
			options={store.clusterOptions}
			value={store.cluster}
			onChange={handleClusterChange}
			id="templates-cluster"
		/>
	{/snippet}
</PageHeader>

{#if store.loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton columns={6} />
{:else if store.error}
	<p role="alert" class="text-destructive">{store.error}</p>
{:else}
	<div role="status" aria-live="polite" class="sr-only">
		{m['admin.templates.templatesLoaded']({ count: store.filteredTemplates.length })}
	</div>

	{#if store.toggleError}
		<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">
			{store.toggleError}
		</p>
	{/if}

	{#if store.templates.length === 0}
		<EmptyState
			title={m['admin.templates.emptyTitle']()}
			description={m['admin.templates.emptyDescription']()}
		>
			{#snippet actions()}
				<Button
					variant="secondary"
					size="sm"
					onclick={() => goto(resolve('/admin/clusters'))}
				>
					{m['admin.templates.emptyAction']()}
				</Button>
			{/snippet}
		</EmptyState>
	{:else if store.filteredTemplates.length === 0}
		<EmptyState
			title={m['admin.templates.noMatchTitle']()}
			description={m['admin.templates.noMatchDescription']()}
		>
			{#snippet actions()}
				<Button
					variant="secondary"
					size="sm"
					onclick={() => store.resetTemplateFilters()}
				>
					{m['admin.templates.resetFilters']()}
				</Button>
			{/snippet}
		</EmptyState>
	{:else}
		<TableCard>
			{#snippet toolbar()}
				<TemplatesTableToolbar {store} />
			{/snippet}
			<TemplatesTable
				templates={store.filteredTemplates}
				toggling={store.toggling}
				onToggle={handleToggle}
				onRemove={handleRemove}
				onEdit={handleEdit}
				sortBy={store.templateSortBy}
				sortDir={store.templateSortDir}
				onSort={handleSort}
			/>
		</TableCard>
	{/if}
{/if}

<TemplateEditDialog
	template={editing}
	open={editing !== null}
	saving={store.toggling?.startsWith('template:') ?? false}
	onClose={() => (editing = null)}
	onSave={handleEditSave}
/>
