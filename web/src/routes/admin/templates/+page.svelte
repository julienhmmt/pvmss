<script lang="ts">
	import { onMount } from 'svelte';
	import {
		setAdminCatalogContext,
		type AdminTemplate,
		type AdminTemplatePatch
	} from '$lib/features/admin-catalog/admin-catalog.svelte';
	import CatalogListStates from '$lib/features/admin-catalog/CatalogListStates.svelte';
	import CatalogToolbar from '$lib/features/admin-catalog/CatalogToolbar.svelte';
	import TemplateEditDialog from '$lib/features/admin-catalog/TemplateEditDialog.svelte';
	import TemplatesTable from '$lib/features/admin-catalog/TemplatesTable.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import TableCard from '$lib/shared/ui/TableCard.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = setAdminCatalogContext();
	const toast = getToastContext();

	let editing = $state<AdminTemplate | null>(null);

	onMount(() => {
		void store.loadTemplates();
	});

	async function handleToggle(vmid: number, enabled: boolean): Promise<void> {
		try {
			await store.toggleTemplate(vmid, enabled);
			toast.success(
				enabled ? m['admin.templates.enabledSuccess']({ vmid }) : m['admin.templates.disabledSuccess']({ vmid })
			);
		} catch {
			toast.error(m['admin.catalog.toggleTemplateError']());
		}
	}

	async function handleRemove(vmid: number): Promise<void> {
		try {
			await store.removeTemplate(vmid);
			toast.success(m['admin.templates.removeSuccess']({ vmid }));
		} catch {
			toast.error(m['admin.templates.removeError']());
		}
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

<CatalogListStates
	loading={store.loading}
	error={store.error}
	toggleError={store.toggleError}
	columns={6}
	totalCount={store.templates.length}
	filteredCount={store.filteredTemplates.length}
	loadedLabel={m['admin.templates.templatesLoaded']({ count: store.filteredTemplates.length })}
	emptyTitle={m['admin.templates.emptyTitle']()}
	emptyDescription={m['admin.templates.emptyDescription']()}
	emptyActionLabel={m['admin.templates.emptyAction']()}
	noMatchTitle={m['admin.templates.noMatchTitle']()}
	noMatchDescription={m['admin.templates.noMatchDescription']()}
	resetLabel={m['admin.templates.resetFilters']()}
	onResetFilters={() => store.resetTemplateFilters()}
>
	<TableCard>
		{#snippet toolbar()}
			<CatalogToolbar {store} kind="templates" />
		{/snippet}
		<TemplatesTable
			templates={store.filteredTemplates}
			toggling={store.toggling}
			onToggle={(vmid, e) => void handleToggle(vmid, e)}
			onRemove={(vmid) => void handleRemove(vmid)}
			onEdit={(template) => (editing = template)}
			sortBy={store.templateSortBy}
			sortDir={store.templateSortDir}
			onSort={(column) => store.setTemplateSort(column)}
		/>
	</TableCard>
</CatalogListStates>

<TemplateEditDialog
	template={editing}
	open={editing !== null}
	saving={store.toggling?.startsWith('template:') ?? false}
	onClose={() => (editing = null)}
	onSave={handleEditSave}
/>
