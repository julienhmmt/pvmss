<script lang="ts">
	/**
	 * MachineListToolbar - the machine collection's filter bar (extracted
	 * from MachineList, ui-review-remediation/05): search field, status
	 * filter, live count and the bulk-select toggle. Reads the list store
	 * from context like the rest of the feature; only view-local state
	 * (select mode, the derived count label) arrives as props.
	 */
	import { getVmListContext, type VmStatus } from './list.svelte';
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
