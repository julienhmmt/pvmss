<script lang="ts">
	/**
	 * NodeTableToolbar — search, status filter, enabled filter, and reset for
	 * the admin node catalog. Lives above the table and binds directly to the
	 * AdminCatalogStore node filter state.
	 */
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
		{ value: 'all', label: m['admin.nodes.filterAllEnabled']() },
		{ value: 'enabled', label: m['admin.nodes.filterEnabled']() },
		{ value: 'disabled', label: m['admin.nodes.filterDisabled']() }
	]);
</script>

<div class="mb-4 flex flex-wrap items-center gap-3">
	<TextField
		type="search"
		class="w-full sm:w-64"
		placeholder={m['admin.nodes.searchPlaceholder']()}
		bind:value={store.nodeSearch}
		data-testid="node-search"
	/>
	<Select
		class="w-full sm:w-48"
		placeholder={m['admin.nodes.filterAllStatuses']()}
		options={store.nodeStatusFilterOptions}
		bind:value={store.nodeStatusFilter}
		data-testid="node-status-filter"
	/>
	<Select
		class="w-full sm:w-44"
		options={enabledFilterOptions}
		bind:value={store.nodeEnabledFilter as string}
		data-testid="node-enabled-filter"
	/>
	<Button
		variant="ghost"
		size="sm"
		onclick={() => store.resetNodeFilters()}
		data-testid="node-reset-filters"
	>
		{m['admin.nodes.resetFilters']()}
	</Button>
	<span class="ml-auto text-sm text-muted-foreground" data-testid="node-result-count">
		{m['admin.nodes.resultCount']({ count: store.filteredNodes.length })}
	</span>
</div>
