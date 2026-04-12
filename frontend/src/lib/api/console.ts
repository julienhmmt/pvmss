import { api } from "./client";

/**
 * VNC ticket response from the backend.
 */
export interface VNCTicket {
  /** The VNC ticket string for authentication */
  ticket: string;
  /** The port number for the WebSocket connection */
  port: number;
  /** The Proxmox node where the VM is located */
  node: string;
}

/**
 * Requests a VNC ticket for the specified VM.
 * @param vmid - The VM ID (must be a positive integer)
 * @returns Promise<VNCTicket> - The VNC ticket information
 * @throws Error if vmid is not a positive integer
 */
export async function getVNCTicket(vmid: number): Promise<VNCTicket> {
  if (vmid <= 0) {
    throw new Error("Invalid VM ID: must be a positive integer");
  }
  return api.post<VNCTicket>(`/api/v1/vms/${vmid}/vnc-ticket`);
}

/**
 * Builds the WebSocket URL for VNC console connection.
 * @param vmid - The VM ID
 * @param ticket - The VNC ticket string (must be non-empty)
 * @param port - The WebSocket port number (must be between 1-65535)
 * @param node - The Proxmox node name (must be non-empty)
 * @returns The complete WebSocket URL
 * @throws Error if ticket, port, node, or host/protocol are invalid
 */
export function buildWebSocketURL(
  vmid: number,
  ticket: string,
  port: number,
  node: string,
): string {
  if (vmid <= 0) {
    throw new Error("Invalid VM ID: must be a positive integer");
  }
  if (!ticket || ticket.trim().length === 0) {
    throw new Error("VNC ticket cannot be empty");
  }
  if (port < 1 || port > 65535) {
    throw new Error("Invalid port number: must be between 1 and 65535");
  }
  if (!node || node.trim().length === 0) {
    throw new Error("Node name cannot be empty");
  }

  const backendHost = import.meta.env.VITE_BACKEND_HOST;
  const backendProtocol = import.meta.env.VITE_BACKEND_PROTOCOL;

  // In production, require explicit backend host configuration
  if (!import.meta.env.DEV && !backendHost) {
    throw new Error(
      "VITE_BACKEND_HOST environment variable is required in production. " +
      "Please configure it to point to your backend server."
    );
  }

  // Use configured values or sensible defaults for development
  const host = backendHost ||
    (import.meta.env.DEV ? "localhost:50000" : (typeof window !== "undefined" ? window.location.host : ""));

  if (!host || host.trim().length === 0) {
    throw new Error("Unable to determine backend host. Please set VITE_BACKEND_HOST environment variable.");
  }

  const protocol = backendProtocol ||
    (typeof window !== "undefined" && window.location.protocol === "https:" ? "wss:" : "ws:");

  if (protocol !== "ws:" && protocol !== "wss:") {
    throw new Error(`Invalid WebSocket protocol: ${protocol}. Must be "ws:" or "wss:"`);
  }

  return (
    `${protocol}//${host}/api/v1/vms/${vmid}/console/websocket` +
    `?port=${port}&node=${encodeURIComponent(node)}&vncticket=${encodeURIComponent(ticket)}`
  );
}
