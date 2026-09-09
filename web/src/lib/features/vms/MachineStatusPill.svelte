<script lang="ts">
	/**
	 * MachineStatusPill — wraps `Pill.svelte` (not a fork) to map the
	 * 7-state `MachineDisplayStatus` (issue 09) to an existing Pill tone
	 * + a Paraglide label. DESIGN.md §8 "Status pills".
	 *
	 * Tone mapping:
	 *   running       → ok (success-soft)
	 *   stopped       → off (muted)
	 *   provisioning   → warn
	 *   starting      → warn
	 *   stopping      → warn
	 *   partial       → warn
	 *   failed        → error (destructive-soft)
	 *
	 * `pending` is forwarded to Pill so the dot pulses for in-flight states.
	 */
	import Pill from '$lib/shared/ui/Pill.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import type { MachineDisplayStatus } from './display-status';

	type Tone = 'ok' | 'off' | 'warn' | 'error';

	interface Props {
		status: MachineDisplayStatus;
		pending?: boolean;
		size?: 'sm' | 'md';
	}

	let { status, pending = false, size = 'sm' }: Props = $props();

	const map: Record<MachineDisplayStatus, { tone: Tone; label: () => string }> = {
		running: { tone: 'ok', label: () => m['machine.status.running']() },
		stopped: { tone: 'off', label: () => m['machine.status.stopped']() },
		provisioning: { tone: 'warn', label: () => m['machine.status.provisioning']() },
		starting: { tone: 'warn', label: () => m['machine.status.starting']() },
		stopping: { tone: 'warn', label: () => m['machine.status.stopping']() },
		partial: { tone: 'warn', label: () => m['machine.status.partial']() },
		failed: { tone: 'error', label: () => m['machine.status.failed']() }
	};

	const view = $derived(map[status]);
</script>

<Pill tone={view.tone} label={view.label()} {pending} {size} />
