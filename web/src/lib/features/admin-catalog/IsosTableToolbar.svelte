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
		{ value: 'all', label: m['admin.isos.filterEnabled']() },
		{ value: 'enabled', label: m['admin.isos.filterEnabledOnly']() },
		{ value: 'disabled', label: m['admin.isos.filterDisabledOnly']() }
	]);
</script>

<div class="mb-4 flex flex-wrap items-center gap-3">
	<TextField
		type="search"
		class="w-full sm:w-64"
		placeholder={m['admin.isos.searchPlaceholder']()}
		bind:value={store.isoSearch}
		data-testid="iso-search"
	/>
	<Select
		class="w-full sm:w-44"
		placeholder={m['admin.isos.filterStorage']()}
		options={store.isoStorageOptions}
		bind:value={store.isoStorageFilter}
		data-testid="iso-storage-filter"
	/>
	<Select
		class="w-full sm:w-44"
		placeholder={m['admin.isos.filterNode']()}
		options={store.isoNodeOptions}
		bind:value={store.isoNodeFilter}
		data-testid="iso-node-filter"
	/>
	<Select
		class="w-full sm:w-44"
		options={enabledFilterOptions}
		bind:value={store.isoEnabledFilter as string}
		data-testid="iso-enabled-filter"
	/>
	<Button
		variant="ghost"
		size="sm"
		onclick={() => store.resetISOFilters()}
		data-testid="iso-reset-filters"
	>
		{m['admin.isos.resetFilters']()}
	</Button>
	<span class="ml-auto text-sm text-muted-foreground" data-testid="iso-result-count">
		{m['admin.isos.resultCount']({ count: store.filteredIsos.length })}
	</span>
</div>
