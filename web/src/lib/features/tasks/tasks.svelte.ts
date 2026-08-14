import { getContext, setContext } from 'svelte';
import { get, ApiRequestError } from '$lib/shared/api/client';
import { m } from '$lib/paraglide/messages.js';

export type TaskKind = 'vm_create' | 'vm_snapshot_create' | 'vm_snapshot_rollback' | 'vm_snapshot_delete';

export interface TrackedTask {
	upid: string;
	kind: TaskKind;
	vmid: number;
	name: string;
}

export interface TaskStatusResponse {
	upid: string;
	state: 'running' | 'ok' | 'error';
	log: string[];
	exitMessage?: string;
}

export interface TaskToast {
	kind: 'success' | 'error';
	message: string;
}

const POLL_INTERVAL_MS = 1500;

/**
 * Global active-task tray state (FR-015): creation dispatches register here
 * and are polled until a terminal state, which fires exactly one toast and
 * removes the entry (FR-016). One instance per app shell, provided through
 * context (constitution VII: no module singletons).
 */
export class TaskTrayStore {
	tasks = $state.raw<readonly TrackedTask[]>([]);
	toast = $state.raw<TaskToast | null>(null);

	#timer: ReturnType<typeof setInterval> | null = null;
	#okListeners: (() => void)[] = [];

	/** Registers a listener fired when any tracked task completes
	 *  successfully — the VM list uses it to pick up creations without a
	 *  manual reload (US1 scenario 3, FR-018's client half). */
	onTaskOk(listener: () => void): () => void {
		this.#okListeners.push(listener);
		return () => {
			this.#okListeners = this.#okListeners.filter((fn) => fn !== listener);
		};
	}

	/** Registers a freshly accepted task and starts polling. */
	track(task: TrackedTask): void {
		this.tasks = [...this.tasks, task];
		this.#startPolling();
	}

	/** Dismisses the current toast (auto-dismiss is the component's concern). */
	clearToast(): void {
		this.toast = null;
	}

	/** Shows a toast unrelated to a task (e.g. the draft-restored notice, V10). */
	notify(toast: TaskToast): void {
		this.toast = toast;
	}

	/** Stops polling — called by the shell when it unmounts. */
	destroy(): void {
		this.#stopPolling();
	}

	#startPolling(): void {
		if (this.#timer !== null) return;
		this.#timer = setInterval(() => void this.#pollAll(), POLL_INTERVAL_MS);
	}

	#stopPolling(): void {
		if (this.#timer !== null) clearInterval(this.#timer);
		this.#timer = null;
	}

	async #pollAll(): Promise<void> {
		const pending = this.tasks;
		if (pending.length === 0) {
			this.#stopPolling();
			return;
		}
		for (const task of pending) {
			await this.#pollOne(task);
		}
	}

	async #pollOne(task: TrackedTask): Promise<void> {
		try {
			const status = await get<TaskStatusResponse>(`/api/v1/tasks/${encodeURIComponent(task.upid)}`);
			if (status.state === 'running') return;
			this.#finish(task, taskToast(task, status));
		} catch (error: unknown) {
			// A 404 means the task is unknown/expired server-side — stop
			// tracking it rather than polling forever (edge case: the tray is
			// tab-local anyway, V11).
			if (error instanceof ApiRequestError && error.status === 404) {
				this.#finish(task, { kind: 'error', message: m['task.taskNoLongerKnown']({ name: task.name }) });
			}
		}
	}

	#finish(task: TrackedTask, toast: TaskToast): void {
		this.tasks = this.tasks.filter((pending) => pending.upid !== task.upid);
		this.toast = toast;
		if (toast.kind === 'success') {
			for (const listener of this.#okListeners) listener();
		}
		if (this.tasks.length === 0) this.#stopPolling();
	}
}

function taskToast(task: TrackedTask, status: TaskStatusResponse): TaskToast {
	const labels: Record<TaskKind, { subject: () => string; success: () => string; failure: () => string }> = {
		vm_create: { subject: () => m['task.subjectVm'](), success: () => m['task.successCreated'](), failure: () => m['task.failureCreationFailed']() },
		vm_snapshot_create: { subject: () => m['task.subjectSnapshot'](), success: () => m['task.successCreated'](), failure: () => m['task.failureCreationFailed']() },
		vm_snapshot_rollback: { subject: () => m['task.subjectVm'](), success: () => m['task.successRolledBack'](), failure: () => m['task.failureRollbackFailed']() },
		vm_snapshot_delete: { subject: () => m['task.subjectSnapshot'](), success: () => m['task.successDeleted'](), failure: () => m['task.failureDeletionFailed']() }
	};
	const label = labels[task.kind];
	if (status.state === 'ok') return { kind: 'success', message: `${label.subject()} "${task.name}" ${label.success()}` };
	return { kind: 'error', message: `${label.subject()} "${task.name}" ${label.failure()}: ${status.exitMessage ?? m['task.unknownError']()}` };
}

const TASK_TRAY_CONTEXT_KEY = Symbol('task-tray');

/** Called once, by the app shell layout. */
export function setTaskTrayContext(): TaskTrayStore {
	const store = new TaskTrayStore();
	setContext(TASK_TRAY_CONTEXT_KEY, store);
	return store;
}

export function getTaskTrayContext(): TaskTrayStore {
	return getContext<TaskTrayStore>(TASK_TRAY_CONTEXT_KEY);
}
