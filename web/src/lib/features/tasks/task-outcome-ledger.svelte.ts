import { getContext, setContext } from 'svelte';
import { SvelteMap } from 'svelte/reactivity';

/**
 * TaskOutcomeLedger - session-scoped, in-memory record of vm_create
 * terminal outcomes the task tray no longer tracks (the tray removes a
 * task the moment it reaches a terminal state).
 *
 * Two outcomes are recorded:
 *   - `partial` - the create API returned `cloudInitPushError` (the VM
 *     exists but access config failed). Written by the create flow at
 *     submit time.
 *   - `failed` - a tracked `vm_create` task ended in `error` (the VM was
 *     not successfully allocated). Written by the shell from the tray's
 *     `onTaskError` signal.
 *
 * ponytail: ephemeral - a reload clears the ledger, so a `partial` VM
 * collapses to `stopped` and a `failed` creation to absent. Upgrade path:
 * persist `partial` server-side (a tag or a status extension) so the
 * no-duplicate safety and the "do not create a duplicate" banner survive a
 * reload. The ceiling is acceptable for now because the dangerous window is
 * the creation session itself, not a later reload.
 */
export type TaskOutcome = 'failed' | 'partial';

const key = (cluster: string, vmid: number): string => `${cluster}:${vmid}`;

export class TaskOutcomeLedger {
	#entries = new SvelteMap<string, TaskOutcome>();
	/** Optional technical detail per outcome (e.g. the cloud-init push
	 *  error), shown to the user as "details for your administrator". */
	#details = new SvelteMap<string, string>();

	get(cluster: string, vmid: number): TaskOutcome | undefined {
		return this.#entries.get(key(cluster, vmid));
	}

	/** The technical detail recorded with the outcome, if any. */
	detail(cluster: string, vmid: number): string | undefined {
		return this.#details.get(key(cluster, vmid));
	}

	record(cluster: string, vmid: number, outcome: TaskOutcome, detail?: string): void {
		this.#entries.set(key(cluster, vmid), outcome);
		if (detail) this.#details.set(key(cluster, vmid), detail);
		else this.#details.delete(key(cluster, vmid));
	}

	/** Clears a recorded outcome once the VM has moved past it (e.g. a
	 *  `partial` VM the user successfully reconfigured, or any VM the user
	 *  deleted). Idempotent. */
	clear(cluster: string, vmid: number): void {
		this.#entries.delete(key(cluster, vmid));
		this.#details.delete(key(cluster, vmid));
	}
}

const TASK_OUTCOME_LEDGER_CONTEXT_KEY = Symbol('task-outcome-ledger');

/** Called once, by the app shell layout (alongside the task tray). */
export function setTaskOutcomeLedgerContext(): TaskOutcomeLedger {
	const ledger = new TaskOutcomeLedger();
	setContext(TASK_OUTCOME_LEDGER_CONTEXT_KEY, ledger);
	return ledger;
}

export function getTaskOutcomeLedgerContext(): TaskOutcomeLedger {
	return getContext<TaskOutcomeLedger>(TASK_OUTCOME_LEDGER_CONTEXT_KEY);
}
