<script lang="ts">
	/**
	 * ClusterSelector - the cluster picker shown in page headers. A small
	 * uppercase "Cluster" label sits beside a bordered select (chevron, cluster
	 * name) built on the shared Select primitive with its `strong` surface, so
	 * it reads as a control on the page background and matches the search
	 * field's border and fill. The external API is unchanged: every call site
	 * keeps passing options/value/onChange/id/label/includeAll/disabled.
	 *
	 * `includeAll` prepends a real, selectable "All clusters" option (value
	 * `''`) to the list - the same technique the VM list's own node filter
	 * already uses - rather than Select's `placeholder` prop, which renders a
	 * disabled "choose one" prompt, a different semantic than "all of them".
	 */
	import type { ClusterOption } from '#lib/shared/clusters.js';
	import { m } from '#lib/paraglide/messages.js';
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
	<div class="flex items-center gap-2">
		<label for={id} class="text-xs font-semibold tracking-[0.08em] text-muted-foreground-subtle uppercase">{label}</label>
		<Select {id} {value} options={selectOptions} onchange={handleChange} {disabled} strong class="w-auto min-w-[11rem]" />
	</div>
{/if}
