<script lang="ts">
	/**
	 * CatalogToolbar - the filter row above every admin catalog table.
	 * One component, six kinds: each catalog page used to ship its own
	 * ~55-line toolbar with the same search + selects + reset + count
	 * shape. The kind config carries the store accessors and message keys;
	 * the markup is written once.
	 */
	import { AdminCatalogStore } from './admin-catalog.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages.js';

	type CatalogKind = 'nodes' | 'bridges' | 'storages' | 'isos' | 'images' | 'templates';

	type Store = AdminCatalogStore;
	type SelectOption = { value: string; label: string };

	interface SelectSpec {
		/** Options from the store, or a static translated list. */
		options: (store: Store) => readonly (string | SelectOption)[];
		get: (store: Store) => string;
		set: (store: Store, value: string) => void;
		/** Disabled first option - "all" style prompt. Omit for a labelled list. */
		placeholder?: () => string;
		testid: string;
		class?: string;
	}

	interface KindConfig {
		search: {
			get: (store: Store) => string;
			set: (store: Store, value: string) => void;
			placeholder: () => string;
			testid: string;
		};
		selects: SelectSpec[];
		reset: (store: Store) => void;
		resetLabel: () => string;
		resetTestId: string;
		count: (store: Store) => number;
		countLabel: (count: number) => string;
		countTestId: string;
	}

	type EnabledFilter = 'all' | 'enabled' | 'disabled';
	type ActiveFilter = 'all' | 'active' | 'inactive';

	/** The enabled/disabled select every catalog shares; only the labels differ. */
	function enabledSelect(
		get: (store: Store) => EnabledFilter,
		set: (store: Store, value: EnabledFilter) => void,
		labels: [() => string, () => string, () => string],
		testid: string
	): SelectSpec {
		return {
			options: () => [
				{ value: 'all', label: labels[0]() },
				{ value: 'enabled', label: labels[1]() },
				{ value: 'disabled', label: labels[2]() }
			],
			get,
			set: (store, value) => set(store, value as EnabledFilter),
			testid
		};
	}

	const CONFIG: Record<CatalogKind, KindConfig> = {
		nodes: {
			search: {
				get: (s) => s.nodeSearch,
				set: (s, v) => (s.nodeSearch = v),
				placeholder: () => m['admin.nodes.searchPlaceholder'](),
				testid: 'node-search'
			},
			selects: [
				{
					options: (s) => s.nodeStatusFilterOptions,
					get: (s) => s.nodeStatusFilter,
					set: (s, v) => (s.nodeStatusFilter = v),
					placeholder: () => m['admin.nodes.filterAllStatuses'](),
					testid: 'node-status-filter',
					class: 'w-full sm:w-48'
				},
				enabledSelect(
					(s) => s.nodeEnabledFilter,
					(s, v) => (s.nodeEnabledFilter = v),
					[
						() => m['admin.nodes.filterAllEnabled'](),
						() => m['admin.nodes.filterEnabled'](),
						() => m['admin.nodes.filterDisabled']()
					],
					'node-enabled-filter'
				)
			],
			reset: (s) => s.resetNodeFilters(),
			resetLabel: () => m['admin.nodes.resetFilters'](),
			resetTestId: 'node-reset-filters',
			count: (s) => s.filteredNodes.length,
			countLabel: (n) => m['admin.nodes.resultCount']({ count: n }),
			countTestId: 'node-result-count'
		},
		bridges: {
			search: {
				get: (s) => s.bridgeSearch,
				set: (s, v) => (s.bridgeSearch = v),
				placeholder: () => m['admin.bridges.searchPlaceholder'](),
				testid: 'bridge-search'
			},
			selects: [
				{
					options: (s) => s.bridgeNodeOptions,
					get: (s) => s.bridgeNodeFilter,
					set: (s, v) => (s.bridgeNodeFilter = v),
					placeholder: () => m['admin.bridges.filterAllNodes'](),
					testid: 'bridge-node-filter'
				},
				{
					options: () => [
						{ value: 'all', label: m['admin.bridges.filterActive']() },
						{ value: 'active', label: m['admin.bridges.filterActiveOnly']() },
						{ value: 'inactive', label: m['admin.bridges.filterInactiveOnly']() }
					],
					get: (s) => s.bridgeActiveFilter,
					set: (s, v) => (s.bridgeActiveFilter = v as ActiveFilter),
					testid: 'bridge-active-filter'
				},
				enabledSelect(
					(s) => s.bridgeEnabledFilter,
					(s, v) => (s.bridgeEnabledFilter = v),
					[
						() => m['admin.bridges.filterAllEnabled'](),
						() => m['common.enabled'](),
						() => m['common.disabled']()
					],
					'bridge-enabled-filter'
				)
			],
			reset: (s) => s.resetBridgeFilters(),
			resetLabel: () => m['admin.bridges.resetFilters'](),
			resetTestId: 'bridge-reset-filters',
			count: (s) => s.filteredBridges.length,
			countLabel: (n) => m['admin.bridges.resultCount']({ count: n }),
			countTestId: 'bridge-result-count'
		},
		storages: {
			search: {
				get: (s) => s.storageSearch,
				set: (s, v) => (s.storageSearch = v),
				placeholder: () => m['admin.storages.searchPlaceholder'](),
				testid: 'storage-search'
			},
			selects: [
				{
					options: (s) => s.storageNodeOptions,
					get: (s) => s.storageNodeFilter,
					set: (s, v) => (s.storageNodeFilter = v),
					placeholder: () => m['admin.storages.filterAllNodes'](),
					testid: 'storage-node-filter'
				},
				{
					options: (s) => s.storageTypeOptions,
					get: (s) => s.storageTypeFilter,
					set: (s, v) => (s.storageTypeFilter = v),
					placeholder: () => m['admin.storages.filterAllTypes'](),
					testid: 'storage-type-filter'
				},
				enabledSelect(
					(s) => s.storageEnabledFilter,
					(s, v) => (s.storageEnabledFilter = v),
					[
						() => m['admin.storages.filterAllEnabled'](),
						() => m['common.enabled'](),
						() => m['common.disabled']()
					],
					'storage-enabled-filter'
				)
			],
			reset: (s) => s.resetStorageFilters(),
			resetLabel: () => m['admin.storages.resetFilters'](),
			resetTestId: 'storage-reset-filters',
			count: (s) => s.filteredStorageCount,
			countLabel: (n) => m['admin.storages.resultCount']({ count: n }),
			countTestId: 'storage-result-count'
		},
		isos: {
			search: {
				get: (s) => s.isoSearch,
				set: (s, v) => (s.isoSearch = v),
				placeholder: () => m['admin.isos.searchPlaceholder'](),
				testid: 'iso-search'
			},
			selects: [
				{
					options: (s) => s.isoStorageOptions,
					get: (s) => s.isoStorageFilter,
					set: (s, v) => (s.isoStorageFilter = v),
					placeholder: () => m['admin.isos.filterStorage'](),
					testid: 'iso-storage-filter'
				},
				{
					options: (s) => s.isoNodeOptions,
					get: (s) => s.isoNodeFilter,
					set: (s, v) => (s.isoNodeFilter = v),
					placeholder: () => m['admin.isos.filterNode'](),
					testid: 'iso-node-filter'
				},
				enabledSelect(
					(s) => s.isoEnabledFilter,
					(s, v) => (s.isoEnabledFilter = v),
					[
						() => m['admin.isos.filterEnabled'](),
						() => m['admin.isos.filterEnabledOnly'](),
						() => m['admin.isos.filterDisabledOnly']()
					],
					'iso-enabled-filter'
				)
			],
			reset: (s) => s.resetISOFilters(),
			resetLabel: () => m['admin.isos.resetFilters'](),
			resetTestId: 'iso-reset-filters',
			count: (s) => s.filteredIsos.length,
			countLabel: (n) => m['admin.isos.resultCount']({ count: n }),
			countTestId: 'iso-result-count'
		},
		images: {
			search: {
				get: (s) => s.imageSearch,
				set: (s, v) => (s.imageSearch = v),
				placeholder: () => m['admin.images.searchPlaceholder'](),
				testid: 'image-search'
			},
			selects: [
				{
					options: (s) => s.imageStorageOptions,
					get: (s) => s.imageStorageFilter,
					set: (s, v) => (s.imageStorageFilter = v),
					placeholder: () => m['admin.images.filterStorage'](),
					testid: 'image-storage-filter'
				},
				{
					options: (s) => s.imageNodeOptions,
					get: (s) => s.imageNodeFilter,
					set: (s, v) => (s.imageNodeFilter = v),
					placeholder: () => m['admin.images.filterNode'](),
					testid: 'image-node-filter'
				},
				enabledSelect(
					(s) => s.imageEnabledFilter,
					(s, v) => (s.imageEnabledFilter = v),
					[
						() => m['admin.images.filterEnabled'](),
						() => m['admin.images.filterEnabledOnly'](),
						() => m['admin.images.filterDisabledOnly']()
					],
					'image-enabled-filter'
				)
			],
			reset: (s) => s.resetImageFilters(),
			resetLabel: () => m['admin.images.resetFilters'](),
			resetTestId: 'image-reset-filters',
			count: (s) => s.filteredImages.length,
			countLabel: (n) => m['admin.images.resultCount']({ count: n }),
			countTestId: 'image-result-count'
		},
		templates: {
			search: {
				get: (s) => s.templateSearch,
				set: (s, v) => (s.templateSearch = v),
				placeholder: () => m['admin.templates.searchPlaceholder'](),
				testid: 'template-search'
			},
			selects: [
				{
					options: (s) => s.templateStorageOptions,
					get: (s) => s.templateStorageFilter,
					set: (s, v) => (s.templateStorageFilter = v),
					placeholder: () => m['admin.templates.filterStorage'](),
					testid: 'template-storage-filter'
				},
				{
					options: (s) => s.templateNodeOptions,
					get: (s) => s.templateNodeFilter,
					set: (s, v) => (s.templateNodeFilter = v),
					placeholder: () => m['admin.templates.filterNode'](),
					testid: 'template-node-filter'
				}
			],
			reset: (s) => s.resetTemplateFilters(),
			resetLabel: () => m['admin.templates.resetFilters'](),
			resetTestId: 'template-reset-filters',
			count: (s) => s.filteredTemplates.length,
			countLabel: (n) => m['admin.templates.resultCount']({ count: n }),
			countTestId: 'template-result-count'
		}
	};

	interface Props {
		store: Store;
		kind: CatalogKind;
	}

	let { store, kind }: Props = $props();
	const config = $derived(CONFIG[kind]);
</script>

<TextField
	type="search"
	class="w-full sm:w-64"
	placeholder={config.search.placeholder()}
	value={config.search.get(store)}
	oninput={(event: Event) => config.search.set(store, (event.currentTarget as HTMLInputElement).value)}
	data-testid={config.search.testid}
/>
{#each config.selects as select (select.testid)}
	<Select
		class={select.class ?? 'w-full sm:w-44'}
		placeholder={select.placeholder?.()}
		options={select.options(store)}
		value={select.get(store)}
		onchange={(event: Event) => select.set(store, (event.currentTarget as HTMLSelectElement).value)}
		data-testid={select.testid}
	/>
{/each}
<Button
	variant="ghost"
	size="sm"
	onclick={() => config.reset(store)}
	data-testid={config.resetTestId}
>
	{config.resetLabel()}
</Button>
<span class="ml-auto text-sm text-muted-foreground" data-testid={config.countTestId}>
	{config.countLabel(config.count(store))}
</span>
