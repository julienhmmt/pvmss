import { getTaskStatus } from "$lib/api/tasks";
import { ApiRequestError } from "$lib/types/api";

export type TaskKind = "vmCreate" | "vmAction";

export type ActiveTask = {
  id: string;
  kind: TaskKind;
  /** Proxmox UPID */
  upid: string;
  node: string;
  /** VMID associated with this task */
  vmid: number;
  label: string;
  status: "running" | "stopped" | "error";
  exitStatus: string;
  startedAt: Date;
};

interface TasksState {
  tasks: ActiveTask[];
}

interface TasksStore {
  readonly tasks: ActiveTask[];
  readonly activeTasks: ActiveTask[];
  add(task: Omit<ActiveTask, "id" | "status" | "exitStatus" | "startedAt">): string;
  remove(id: string): void;
  onComplete(id: string, callback: (task: ActiveTask) => void): void;
}

const POLL_INTERVAL_MS = 2500;
const COMPLETED_TTL_MS = 5 * 60 * 1000;
const MAX_CONSECUTIVE_ERRORS = 5;
const MAX_POLL_DURATION_MS = 10 * 60 * 1000; // matches backend finalizeAfterTask timeout

function createTasksStore(): TasksStore {
  let state = $state<TasksState>({ tasks: [] });

  const completionCallbacks = new Map<string, (task: ActiveTask) => void>();
  const pollingIntervals = new Map<string, ReturnType<typeof setInterval>>();
  const consecutiveErrors = new Map<string, number>();
  const cleanupTimers = new Map<string, ReturnType<typeof setTimeout>>();

  function updateTask(id: string, patch: Partial<ActiveTask>) {
    const current = state.tasks.find((t) => t.id === id);
    if (!current) return;
    const changed = (Object.keys(patch) as (keyof ActiveTask)[]).some(
      (key) => current[key] !== patch[key],
    );
    if (!changed) return;
    state.tasks = state.tasks.map((t) => (t.id === id ? { ...t, ...patch } : t));
  }

  function stopPolling(taskId: string) {
    const interval = pollingIntervals.get(taskId);
    if (interval) {
      clearInterval(interval);
      pollingIntervals.delete(taskId);
    }
    consecutiveErrors.delete(taskId);
  }

  function scheduleCleanup(taskId: string) {
    const timer = setTimeout(() => {
      cleanupTimers.delete(taskId);
      stopPolling(taskId);
      completionCallbacks.delete(taskId);
      state.tasks = state.tasks.filter((t) => t.id !== taskId);
    }, COMPLETED_TTL_MS);
    cleanupTimers.set(taskId, timer);
  }

  function startPolling(taskId: string) {
    const interval = setInterval(async () => {
      const current = state.tasks.find((t) => t.id === taskId);
      if (!current) {
        stopPolling(taskId);
        return;
      }

      // Guard against polling forever on an unrecognized Proxmox status
      if (Date.now() - current.startedAt.getTime() > MAX_POLL_DURATION_MS) {
        stopPolling(taskId);
        updateTask(taskId, {
          status: "error",
          exitStatus: "timeout",
        });
        return;
      }

      try {
        const result = await getTaskStatus(current.node, current.upid);
        consecutiveErrors.set(taskId, 0);

        if (result.status === "stopped") {
          stopPolling(taskId);

          const isError = result.exitStatus !== "OK";
          updateTask(taskId, {
            status: isError ? "error" : "stopped",
            exitStatus: result.exitStatus,
          });

          const cb = completionCallbacks.get(taskId);
          if (cb) {
            const updated = state.tasks.find((t) => t.id === taskId);
            if (updated) cb(updated);
            completionCallbacks.delete(taskId);
          }

          // Auto-cleanup completed tasks after TTL
          scheduleCleanup(taskId);
        }
      } catch (err: unknown) {
        // Stop polling on auth failures — the session is gone
        if (err instanceof ApiRequestError && (err.status === 401 || err.status === 403)) {
          stopPolling(taskId);
          updateTask(taskId, {
            status: "error",
            exitStatus: "authFailed",
          });
          return;
        }

        // Stop polling after too many consecutive errors
        const errors = (consecutiveErrors.get(taskId) ?? 0) + 1;
        consecutiveErrors.set(taskId, errors);
        if (errors >= MAX_CONSECUTIVE_ERRORS) {
          stopPolling(taskId);
          updateTask(taskId, {
            status: "error",
            exitStatus: "pollingFailed",
          });
        }
      }
    }, POLL_INTERVAL_MS);

    pollingIntervals.set(taskId, interval);
  }

  return {
    get tasks() {
      return state.tasks;
    },
    get activeTasks() {
      return state.tasks.filter((t) => t.status === "running");
    },

    add(task) {
      const id = crypto.randomUUID();
      const newTask: ActiveTask = {
        ...task,
        id,
        status: "running",
        exitStatus: "",
        startedAt: new Date(),
      };
      state.tasks = [...state.tasks, newTask];
      startPolling(id);
      return id;
    },

    remove(id) {
      stopPolling(id);
      completionCallbacks.delete(id);
      const timer = cleanupTimers.get(id);
      if (timer) {
        clearTimeout(timer);
        cleanupTimers.delete(id);
      }
      state.tasks = state.tasks.filter((t) => t.id !== id);
    },

    onComplete(id, callback) {
      const task = state.tasks.find((t) => t.id === id);
      if (!task) return;
      if (task.status !== "running") {
        callback(task);
        return;
      }
      completionCallbacks.set(id, callback);
    },
  };
}

export const tasks = createTasksStore();
