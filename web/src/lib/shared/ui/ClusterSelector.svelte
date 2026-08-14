<script lang="ts">
	import type { ClusterOption } from '$lib/shared/clusters';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		options: readonly ClusterOption[];
		value: string;
		onChange: (value: string) => void;
		id?: string;
		label?: string;
		includeAll?: boolean;
	}

	let { options, value, onChange, id = 'cluster-selector', label = m['common.cluster'](), includeAll = false }: Props = $props();

	function handleChange(event: Event): void {
		onChange((event.currentTarget as HTMLSelectElement).value);
	}
</script>

{#if options.length > 1}
	<label for={id} class="flex items-center gap-2 text-sm font-medium">
		{label}
		<select
			{id}
			class="rounded-md border border-input bg-background px-3 py-1.5 text-sm font-normal"
			value={value}
			onchange={handleChange}
			aria-label={label}
		>
			{#if includeAll}
				<option value="">{m['common.allClusters']()}</option>
			{/if}
			{#each options as option (option.name)}
				<option value={option.name}>{option.name}</option>
			{/each}
		</select>
	</label>
{/if}
