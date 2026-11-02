<script lang="ts">
	/**
	 * MachineStatusPill - wraps `Pill.svelte` (not a fork) and reads the
	 * shared machine-status vocabulary (`machineStatusView` in
	 * display-status.ts): one map for the 8-state `MachineDisplayStatus`,
	 * the canonical `machine.status.*` labels, DESIGN.md §8 "Status pills".
	 *
	 * `pending` is forwarded to Pill so the dot pulses for in-flight states.
	 */
	import Pill from '#lib/shared/ui/Pill.svelte';
	import { machineStatusView, type MachineDisplayStatus } from './display-status';

	interface Props {
		status: MachineDisplayStatus;
		pending?: boolean;
		size?: 'sm' | 'md';
	}

	let { status, pending = false, size = 'sm' }: Props = $props();

	const view = $derived(machineStatusView(status));
</script>

<Pill tone={view.tone} label={view.label()} {pending} {size} />
