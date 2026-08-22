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

	const activeFilterOptions = $derived([
		{ value: 'all', label: m['admin.bridges.filterActive']() },
		{ value: 'active', label: m['admin.bridges.filterActiveOnly']() },
		{ value: 'inactive', label: m['admin.bridges.filterInactiveOnly']() }
	]);

	const enabledFilterOptions = $derived([
		{ value: 'all', label: m['admin.bridges.filterAllEnabled']() },
		{ value: 'enabled', label: m['common.enabled']() },
		{ value: 'disabled', label: m['common.disabled']() }
	]);
</script>

<div class="mb-4 flex flex-wrap items-center gap-3">
	<TextField
		type="search"
		class="w-full sm:w-64"
		placeholder={m['admin.bridges.searchPlaceholder']()}
		bind:value={store.bridgeSearch}
		data-testid="bridge-search"
	/>
	<Select
		class="w-full sm:w-44"
		placeholder={m['admin.bridges.filterAllNodes']()}
		options={store.bridgeNodeOptions}
		bind:value={store.bridgeNodeFilter}
		data-testid="bridge-node-filter"
	/>
	<Select
		class="w-full sm:w-44"
		options={activeFilterOptions}
		bind:value={store.bridgeActiveFilter as string}
		data-testid="bridge-active-filter"
	/>
	<Select
		class="w-full sm:w-44"
		options={enabledFilterOptions}
		bind:value={store.bridgeEnabledFilter as string}
		data-testid="bridge-enabled-filter"
	/>
	<Button
		variant="ghost"
		size="sm"
		onclick={() => store.resetBridgeFilters()}
		data-testid="bridge-reset-filters"
	>
		{m['admin.bridges.resetFilters']()}
	</Button>
	<span class="ml-auto text-sm text-muted-foreground" data-testid="bridge-result-count">
		{m['admin.bridges.resultCount']({ count: store.filteredBridges.length })}
	</span>
</div>
