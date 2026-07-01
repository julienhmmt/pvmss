import type { StatusIndicator } from "$lib/types/navbar";

interface StatusState {
  proxmoxConnection: StatusIndicator;
  clusterHealth: StatusIndicator;
}

interface StatusStore {
  proxmoxConnection: StatusIndicator;
  clusterHealth: StatusIndicator;
  updateProxmoxConnection(
    status: StatusIndicator["status"],
    tooltip?: string,
  ): void;
  updateClusterHealth(
    status: StatusIndicator["status"],
    tooltip?: string,
  ): void;
}

function createStatusStore(): StatusStore {
  const state = $state<StatusState>({
    proxmoxConnection: {
      id: "proxmox",
      name: "Proxmox",
      status: "unknown",
      tooltip: "Connection status unknown",
    },
    clusterHealth: {
      id: "cluster",
      name: "Cluster",
      status: "unknown",
      tooltip: "Cluster health unknown",
    },
  });

  return {
    get proxmoxConnection() {
      return state.proxmoxConnection;
    },
    get clusterHealth() {
      return state.clusterHealth;
    },

    updateProxmoxConnection(
      status: StatusIndicator["status"],
      tooltip?: string,
    ) {
      state.proxmoxConnection = {
        ...state.proxmoxConnection,
        status,
        tooltip: tooltip || getStatusTooltip(status, "Proxmox"),
      };
    },

    updateClusterHealth(status: StatusIndicator["status"], tooltip?: string) {
      state.clusterHealth = {
        ...state.clusterHealth,
        status,
        tooltip: tooltip || getStatusTooltip(status, "Cluster"),
      };
    },
  };
}

function getStatusTooltip(
  status: StatusIndicator["status"],
  name: string,
): string {
  switch (status) {
    case "connected":
      return `${name} is connected and healthy`;
    case "disconnected":
      return `${name} is disconnected`;
    case "warning":
      return `${name} has warnings`;
    case "unknown":
      return `${name} status unknown`;
    default:
      return `${name} status: ${status}`;
  }
}

export const statusStore = createStatusStore();
