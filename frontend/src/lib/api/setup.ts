import { api } from "./client";
import { transformKeysToCamelCase, transformKeysToSnakeCase } from "$lib/utils/transform";

export interface SetupStatus {
  complete: boolean;
  proxmoxOk: boolean;
  offline: boolean;
  proxmoxUrl: string;
}

export interface SetupConnectionTestResult {
  ok: boolean;
  proxmoxUrl: string;
  error?: string;
}

export interface SetupProxmoxData {
  nodes: string[];
  storages: string[];
  isos: string[];
  vmbrs: string[];
}

export interface SetupLimits {
  maxVms: number;
  maxVmPerUser: number;
  maxNetworkCards: number;
  maxDiskPerVm: number;
  maxSnapshots: number;
  allowCustomYaml: boolean;
}

export interface SetupCompleteRequest {
  enabledNodes: string[];
  enabledStorages: string[];
  enabledIsos: string[];
  enabledVmbrs: string[];
  limits: SetupLimits;
}

export async function getSetupStatus(): Promise<SetupStatus> {
  const res = await api.get<SetupStatus>("/api/v1/setup/status");
  return transformKeysToCamelCase<SetupStatus>(res);
}

export async function testConnection(): Promise<SetupConnectionTestResult> {
  const res = await api.post<SetupConnectionTestResult>("/api/v1/setup/test-connection");
  return transformKeysToCamelCase<SetupConnectionTestResult>(res);
}

export async function getProxmoxData(): Promise<SetupProxmoxData> {
  const res = await api.get<SetupProxmoxData>("/api/v1/setup/proxmox-data");
  return transformKeysToCamelCase<SetupProxmoxData>(res);
}

export async function completeSetup(req: SetupCompleteRequest): Promise<void> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>(req);
  await api.post("/api/v1/setup/complete", payload);
}
