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

	const enabledFilterOptions = $derived([
		{ value: 'all', label: m['admin.storages.filterAllEnabled']() },
		{ value: 'enabled', label: m['common.enabled']() },
		{ value: 'disabled', label: m['common.disabled']() }
	]);
</script>

<div class="flex w-full flex-wrap items-center gap-3">
	<TextField
		type="search"
		class="w-full sm:w-64"
		placeholder={m['admin.storages.searchPlaceholder']()}
		bind:value={store.storageSearch}
		data-testid="storage-search"
	/>
	<Select
		class="w-full sm:w-44"
		placeholder={m['admin.storages.filterAllNodes']()}
		options={store.storageNodeOptions}
		bind:value={store.storageNodeFilter}
		data-testid="storage-node-filter"
	/>
	<Select
		class="w-full sm:w-44"
		placeholder={m['admin.storages.filterAllTypes']()}
		options={store.storageTypeOptions}
		bind:value={store.storageTypeFilter}
		data-testid="storage-type-filter"
	/>
	<Select
		class="w-full sm:w-44"
		options={enabledFilterOptions}
		bind:value={store.storageEnabledFilter as string}
		data-testid="storage-enabled-filter"
	/>
	<Button
		variant="ghost"
		size="sm"
		onclick={() => store.resetStorageFilters()}
		data-testid="storage-reset-filters"
	>
		{m['admin.storages.resetFilters']()}
	</Button>
	<span class="ml-auto text-sm text-muted-foreground" data-testid="storage-result-count">
		{m['admin.storages.resultCount']({ count: store.filteredStorageCount })}
	</span>
</div>
