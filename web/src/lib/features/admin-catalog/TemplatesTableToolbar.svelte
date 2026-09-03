<script lang="ts">
	import { AdminCatalogStore } from './admin-catalog.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		store: AdminCatalogStore;
	}

	let { store }: Props = $props();
</script>

<div class="mb-4 flex flex-wrap items-center gap-3">
	<TextField
		type="search"
		class="w-full sm:w-64"
		placeholder={m['admin.templates.searchPlaceholder']()}
		bind:value={store.templateSearch}
		data-testid="template-search"
	/>
	<Select
		class="w-full sm:w-44"
		placeholder={m['admin.templates.filterStorage']()}
		options={store.templateStorageOptions}
		bind:value={store.templateStorageFilter}
		data-testid="template-storage-filter"
	/>
	<Select
		class="w-full sm:w-44"
		placeholder={m['admin.templates.filterNode']()}
		options={store.templateNodeOptions}
		bind:value={store.templateNodeFilter}
		data-testid="template-node-filter"
	/>
	<Button
		variant="ghost"
		size="sm"
		onclick={() => store.resetTemplateFilters()}
		data-testid="template-reset-filters"
	>
		{m['admin.templates.resetFilters']()}
	</Button>
	<span class="ml-auto text-sm text-muted-foreground" data-testid="template-result-count">
		{m['admin.templates.resultCount']({ count: store.filteredTemplates.length })}
	</span>
</div>
