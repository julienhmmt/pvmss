<script lang="ts">
	/**
	 * CapacityCell - one dimension (VMs, vCPU, RAM, disk) of a node's PVMSS
	 * capacity. A defined cap renders `used / cap` over a NodeUsageBar - the
	 * bar is the distance to refusal of creation. An unset cap (0) renders
	 * the bare usage with "No cap": a bar without a bound would be a lie.
	 * At or above the cap an error pill makes the state non-colour-only.
	 */
	import { m } from '$lib/paraglide/messages.js';
	import NodeUsageBar from '$lib/shared/ui/NodeUsageBar.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';

	interface Props {
		/** Current pvmss-tagged usage of the dimension. */
		used: number;
		/** Configured ceiling; 0 means uncapped. */
		cap: number;
		/** Unit suffix rendered after the numbers (e.g. "vCPU", "GB"). */
		unit?: string;
	}

	let { used, cap, unit = '' }: Props = $props();

	const suffix = $derived(unit === '' ? '' : ` ${unit}`);
	const text = $derived(`${used} / ${cap}${suffix}`);
</script>

{#if cap > 0}
	<div class="flex flex-col items-start gap-1">
		<NodeUsageBar value={used / cap} label={text} />
		{#if used >= cap}
			<Pill tone="error" dot={false} label={m['policy.capReached']()} />
		{/if}
	</div>
{:else}
	<span class="text-muted-foreground">{used}{suffix} · {m['policy.uncapped']()}</span>
{/if}
