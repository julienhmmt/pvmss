import { api } from "./client";

export interface TaskStatus {
  upid: string;
  node: string;
  status: "running" | "stopped" | string;
  exitstatus: string;
}

export interface TaskLogEntry {
  n: number;
  t: string;
}

/** Fetches the current status of a Proxmox task by node + UPID. */
export async function getTaskStatus(
  node: string,
  upid: string,
): Promise<TaskStatus> {
  const params = new URLSearchParams({ node, upid });
  return api.get<TaskStatus>(`/api/v1/tasks/status?${params.toString()}`);
}

/** Fetches the log lines of a Proxmox task by node + UPID. */
export async function getTaskLog(
  node: string,
  upid: string,
): Promise<TaskLogEntry[]> {
  const params = new URLSearchParams({ node, upid });
  return api.get<TaskLogEntry[]>(`/api/v1/tasks/log?${params.toString()}`);
}
