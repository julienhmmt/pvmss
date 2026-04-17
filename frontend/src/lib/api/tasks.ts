import { api } from "./client";
import { transformKeysToCamelCase } from "$lib/utils/transform";

export interface TaskStatus {
  upid: string;
  node: string;
  status: "running" | "stopped" | string;
  exitStatus: string;
}

export interface TaskLogEntry {
  /** Line number in the task log */
  lineNumber: number;
  /** Log line text content */
  text: string;
}

/** Fetches the current status of a Proxmox task by node + UPID. */
export async function getTaskStatus(
  node: string,
  upid: string,
): Promise<TaskStatus> {
  const params = new URLSearchParams({ node, upid });
  const response = await api.get<Record<string, unknown>>(`/api/v1/tasks/status?${params.toString()}`);
  return transformKeysToCamelCase<TaskStatus>(response);
}

/** Fetches the log lines of a Proxmox task by node + UPID. */
export async function getTaskLog(
  node: string,
  upid: string,
): Promise<TaskLogEntry[]> {
  const params = new URLSearchParams({ node, upid });
  const response = await api.get<Record<string, unknown>[]>(`/api/v1/tasks/log?${params.toString()}`);
  return transformKeysToCamelCase<TaskLogEntry[]>(response);
}
