import type { VmStatus } from './list.svelte';
import type { VmAction } from './detail.svelte';
import type { TrackedTask } from '$lib/features/tasks/tasks.svelte';

/**
 * The seven end-user-facing machine states (DESIGN.md §7). These are not
 * server enum values — they are derived client-side from the server status,
 * in-flight power actions, the task tray (vm_create), and a session-scoped
 * outcome ledger. See {@link displayStatus}.
 */
export type MachineDisplayStatus =
	| 'running'
	| 'stopped'
	| 'provisioning'
	| 'starting'
	| 'stopping'
	| 'failed'
	| 'partial';

/** The minimum a caller must provide for the derivation to inspect. */
export interface MachineDisplayInput {
	cluster: string;
	vmid: number;
	/** Server-reported status — the floor; never fabricated. */
	status: VmStatus;
}

/** In-flight vm_create tasks (read from the task tray). A task present here
 *  is in flight — the tray removes terminal tasks, so presence ⇒ running. */
export interface TaskTraySnapshot {
	tasks: readonly TrackedTask[];
}

/**
 * Session-scoped record of vm_create outcomes the tray no longer tracks.
 *
 * ponytail: ephemeral — a reload clears it, so a `partial` VM collapses to
 * `stopped` and a `failed` creation to absent after reload. Upgrade path:
 * persist `partial` server-side (a tag or a status extension) so the
 * no-duplicate safety survives a reload.
 */
export interface TaskOutcomeLedger {
	get(cluster: string, vmid: number): 'failed' | 'partial' | undefined;
}

export interface DisplayStatusDeps {
	/** In-flight vm_create tasks (the tray). */
	tray: TaskTraySnapshot;
	/** Session outcome ledger for failed / partial. Optional — callers
	 *  that only render running / stopped (e.g. admin views) may omit it. */
	ledger?: TaskOutcomeLedger;
	/** In-flight power action for this VM, if any. The list passes its
	 *  per-row map; the detail passes its single in-flight kind. */
	inFlightAction?: VmAction | null;
}

/** Power actions that mean "the VM is heading toward running". */
const STARTING_ACTIONS: ReadonlySet<VmAction> = new Set(['start', 'reboot', 'reset', 'resume']);

/** Power actions that mean "the VM is heading toward stopped". */
const STOPPING_ACTIONS: ReadonlySet<VmAction> = new Set(['shutdown', 'stop']);

/**
 * Derives the end-user display status from the available signals, in
 * priority order:
 *
 * 1. In-flight power action → `starting` / `stopping` (the VM is actively
 *    transitioning; this wins over a stale server status).
 * 2. In-flight `vm_create` task → `provisioning`.
 * 3. Session outcome ledger → `partial` / `failed` (only when the VM is
 *    not running — a live VM is connectable regardless of a stale
 *    creation-time outcome).
 * 4. Server status floor (`running` / `stopped`; `paused` collapses to
 *    `stopped` — it is not a Calm-workspace display state).
 */
export function displayStatus(vm: MachineDisplayInput, deps: DisplayStatusDeps): MachineDisplayStatus {
	const { tray, ledger, inFlightAction = null } = deps;

	if (inFlightAction !== null) {
		if (STARTING_ACTIONS.has(inFlightAction)) return 'starting';
		if (STOPPING_ACTIONS.has(inFlightAction)) return 'stopping';
	}

	for (const task of tray.tasks) {
		if (task.cluster === vm.cluster && task.vmid === vm.vmid && task.kind === 'vm_create') {
			return 'provisioning';
		}
	}

	const outcome = ledger?.get(vm.cluster, vm.vmid);
	// Both outcomes are creation-time signals. Once the server reports the
	// VM is up, the user can reconfigure / connect — the badge returns to
	// `running` so we never contradict a live VM with "Creation failed".
	// A `failed` outcome from a deadline / 404 / poll-error is not a hard
	// guarantee the VM is absent; the server status is the floor.
	if (vm.status !== 'running') {
		if (outcome === 'partial') return 'partial';
		if (outcome === 'failed') return 'failed';
	}

	return vm.status === 'running' ? 'running' : 'stopped';
}

/**
 * A machine is connectable only when it is running AND has a reported
 * address. The UI never fabricates an address (DESIGN.md §7 "No guessed
 * addresses"). On the list, where the DTO carries no address, this returns
 * false and the row falls back to "View details" — honest, not a guess.
 */
export function canConnect(status: MachineDisplayStatus, address: string | undefined | null): boolean {
	return status === 'running' && Boolean(address);
}

/** True for the two states that must not be duplicated via the create form. */
export function isUncertainOutcome(status: MachineDisplayStatus): boolean {
	return status === 'failed' || status === 'partial';
}
