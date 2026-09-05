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
		{ value: 'all', label: m['admin.images.filterEnabled']() },
		{ value: 'enabled', label: m['admin.images.filterEnabledOnly']() },
		{ value: 'disabled', label: m['admin.images.filterDisabledOnly']() }
	]);
</script>

<TextField
	type="search"
	class="w-full sm:w-64"
	placeholder={m['admin.images.searchPlaceholder']()}
	bind:value={store.imageSearch}
	data-testid="image-search"
/>
<Select
	class="w-full sm:w-44"
	placeholder={m['admin.images.filterStorage']()}
	options={store.imageStorageOptions}
	bind:value={store.imageStorageFilter}
	data-testid="image-storage-filter"
/>
<Select
	class="w-full sm:w-44"
	placeholder={m['admin.images.filterNode']()}
	options={store.imageNodeOptions}
	bind:value={store.imageNodeFilter}
	data-testid="image-node-filter"
/>
<Select
	class="w-full sm:w-44"
	options={enabledFilterOptions}
	bind:value={store.imageEnabledFilter as string}
	data-testid="image-enabled-filter"
/>
<Button
	variant="ghost"
	size="sm"
	onclick={() => store.resetImageFilters()}
	data-testid="image-reset-filters"
>
	{m['admin.images.resetFilters']()}
</Button>
<span class="ml-auto text-sm text-muted-foreground" data-testid="image-result-count">
	{m['admin.images.resultCount']({ count: store.filteredImages.length })}
</span>
