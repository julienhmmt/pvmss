export interface SetupStatus {
  complete: boolean;
  proxmox_ok: boolean;
  offline: boolean;
  proxmox_url: string;
}

export interface SetupConnectionTestResult {
  ok: boolean;
  proxmox_url: string;
  error?: string;
}

export interface SetupProxmoxData {
  nodes: string[];
  storages: string[];
  isos: string[];
  vmbrs: string[];
}

export interface SetupLimits {
  max_vms: number;
  max_vm_per_user: number;
  max_network_cards: number;
  max_disk_per_vm: number;
  max_snapshots: number;
  allow_custom_yaml: boolean;
}

export interface SetupCompleteRequest {
  enabled_nodes: string[];
  enabled_storages: string[];
  enabled_isos: string[];
  enabled_vmbrs: string[];
  limits: SetupLimits;
}

export async function getSetupStatus(): Promise<SetupStatus> {
  const res = await fetch("/api/v1/setup/status", { credentials: "same-origin" });
  if (!res.ok) throw new Error("Failed to fetch setup status");
  return res.json();
}

export async function testConnection(): Promise<SetupConnectionTestResult> {
  const res = await fetch("/api/v1/setup/test-connection", {
    method: "POST",
    credentials: "same-origin",
  });
  if (!res.ok) throw new Error("Connection test failed");
  return res.json();
}

export async function getProxmoxData(): Promise<SetupProxmoxData> {
  const res = await fetch("/api/v1/setup/proxmox-data", { credentials: "same-origin" });
  if (!res.ok) throw new Error("Failed to fetch Proxmox data");
  return res.json();
}

export async function completeSetup(req: SetupCompleteRequest): Promise<void> {
  const res = await fetch("/api/v1/setup/complete", {
    method: "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
  if (!res.ok) throw new Error("Failed to complete setup");
}
