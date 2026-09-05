<script lang="ts">
	/**
	 * ClusterSelector — the cluster-picker label+select pair shown in most
	 * page headers. Was the last hand-rolled `<select>` in the app (its own
	 * `rounded-md border border-input px-3 py-1.5`, matching neither the
	 * radius nor the height of every other control); it now wraps the shared
	 * Select primitive. The external API is unchanged on purpose — every call
	 * site keeps passing options/value/onChange/id/label/includeAll/disabled
	 * exactly as before.
	 *
	 * `includeAll` prepends a real, selectable "All clusters" option (value
	 * `''`) to the list — the same technique the VM list's own node filter
	 * already uses — rather than Select's `placeholder` prop, which renders a
	 * disabled "choose one" prompt, a different semantic than "all of them".
	 */
	import type { ClusterOption } from '$lib/shared/clusters';
	import { m } from '$lib/paraglide/messages.js';
	import Select from './Select.svelte';

	interface Props {
		options: readonly ClusterOption[];
		value: string;
		onChange: (value: string) => void;
		id?: string;
		label?: string;
		includeAll?: boolean;
		disabled?: boolean;
	}

	let { options, value, onChange, id = 'cluster-selector', label = m['common.cluster'](), includeAll = false, disabled = false }: Props = $props();

	const selectOptions = $derived([
		...(includeAll ? [{ value: '', label: m['common.allClusters']() }] : []),
		...options.map((option) => ({ value: option.name, label: option.displayName || option.name }))
	]);

	function handleChange(event: Event): void {
		onChange((event.currentTarget as HTMLSelectElement).value);
	}
</script>

{#if options.length > 1}
	<label for={id} class="flex items-center gap-2 text-sm font-medium">
		{label}
		<Select {id} {value} options={selectOptions} onchange={handleChange} {disabled} class="w-auto" />
	</label>
{/if}
