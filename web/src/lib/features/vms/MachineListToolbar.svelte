<script lang="ts">
	/**
	 * MachineListToolbar - the machine collection's filter bar (extracted
	 * from MachineList, ui-review-remediation/05): search field, status
	 * filter, live count and the bulk-select toggle. Reads the list store
	 * from context like the rest of the feature; only view-local state
	 * (select mode, the derived count label) arrives as props.
	 */
	import { getVmListContext, type VmSortBy, type VmStatus } from './list.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import Button from '#lib/shared/ui/Button.svelte';
	import Select from '#lib/shared/ui/Select.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';
	import Toolbar from '#lib/shared/ui/Toolbar.svelte';
	import SearchIcon from '#lib/shared/ui/icons/SearchIcon.svelte';

	/** A server status, or the client-derived "needs attention" pseudo-status. */
	export type StatusFilter = VmStatus | '' | 'attention';
	const ATTENTION_VALUE = 'attention';

	const STATUS_OPTIONS: readonly { value: StatusFilter; label: () => string }[] = [
		{ value: '', label: () => m['vms.list.filterAll']() },
		{ value: 'running', label: () => m['machine.status.running']() },
		{ value: 'stopped', label: () => m['machine.status.stopped']() },
		{ value: ATTENTION_VALUE, label: () => m['vms.list.filterAttention']() }
	];

	/** The sort keys the list offers, in the order the select shows them. */
	const SORT_OPTIONS: readonly { value: VmSortBy; label: () => string }[] = [
		{ value: 'name', label: () => m['vms.list.sortFieldName']() },
		{ value: 'cpu', label: () => m['vms.list.columnCpu']() },
		{ value: 'memory', label: () => m['vms.list.columnMemory']() },
		{ value: 'vmid', label: () => m['vms.list.sortFieldVmid']() },
		{ value: 'status', label: () => m['vms.list.columnStatus']() }
	];

	interface Props {
		selectMode: boolean;
		/** Whether the select-mode toggle is offered (items present). */
		showToggle: boolean;
		/** The derived "N machines" label. */
		countLabel: string;
		onToggleSelectMode: () => void;
	}

	let { selectMode, showToggle, countLabel, onToggleSelectMode }: Props = $props();

	const store = getVmListContext();

	const statusFilter = $derived<StatusFilter>(store.attention ? ATTENTION_VALUE : store.status);

	function setStatusFilter(value: string): void {
		if (value === ATTENTION_VALUE) store.setAttention(true);
		else store.setStatus(value as VmStatus | '');
	}

	const sortDirLabel = $derived(
		store.sortDir === 'asc' ? m['vms.list.sortDirAsc']() : m['vms.list.sortDirDesc']()
	);
</script>

<Toolbar>
	{#snippet search()}
		<label for="vm-search" class="sr-only">{m['common.search']()}</label>
		<TextField
			id="vm-search"
			type="search"
			placeholder={m['vms.list.searchPlaceholder']()}
			value={store.search}
			oninput={(event: Event) => store.applySearch((event.currentTarget as HTMLInputElement).value)}
			data-testid="vm-search"
		>
			{#snippet leading()}<SearchIcon class="h-4 w-4" />{/snippet}
		</TextField>
	{/snippet}

	{#snippet filters()}
		<label class="sr-only" for="vm-status-filter">{m['vms.list.filterStatusLabel']()}</label>
		<Select
			id="vm-status-filter"
			class="w-auto min-w-[9rem]"
			value={statusFilter}
			onchange={(event: Event) => setStatusFilter((event.currentTarget as HTMLSelectElement).value)}
			options={STATUS_OPTIONS.map((option) => ({ value: option.value, label: option.label() }))}
			data-testid="vm-status-filter"
		/>
		<div class="flex items-center gap-1">
			<label class="sr-only" for="vm-sort-field">{m['vms.list.sortLabel']()}</label>
			<Select
				id="vm-sort-field"
				class="w-auto"
				value={store.sortBy}
				onchange={(event: Event) => store.setSort((event.currentTarget as HTMLSelectElement).value as VmSortBy)}
				options={SORT_OPTIONS.map((option) => ({ value: option.value, label: option.label() }))}
				data-testid="vm-sort-field"
			/>
			<!-- Re-clicking the active key is the store's direction toggle. -->
			<Button
				variant="ghost"
				size="icon-sm"
				label={sortDirLabel}
				title={sortDirLabel}
				onclick={() => store.setSort(store.sortBy)}
				data-testid="vm-sort-dir"
			>
				<svg
					viewBox="0 0 12 12"
					class="h-3.5 w-3.5 transition-transform duration-150 {store.sortDir === 'desc' ? 'rotate-180' : ''}"
					fill="none"
					stroke="currentColor"
					stroke-width="1.75"
					stroke-linecap="round"
					stroke-linejoin="round"
					aria-hidden="true"
				>
					<path d="M6 10.5v-9" />
					<path d="M2.75 4.75 6 1.5l3.25 3.25" />
				</svg>
			</Button>
		</div>
	{/snippet}

	{#snippet meta()}
		{#if store.result}
			<span class="tabular-nums" data-testid="vm-count">{countLabel}</span>
		{/if}
	{/snippet}

	{#snippet actions()}
		{#if showToggle}
			<Button
				variant={selectMode ? 'secondary' : 'ghost'}
				size="sm"
				aria-pressed={selectMode}
				title={m['vms.list.selectModeHint']()}
				onclick={onToggleSelectMode}
				data-testid="vm-select-mode"
			>
				{selectMode ? m['vms.list.selectModeDone']() : m['vms.list.selectMode']()}
			</Button>
		{/if}
	{/snippet}
</Toolbar>
