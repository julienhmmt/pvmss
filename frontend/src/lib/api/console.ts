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
  /** Opaque backend token that maps to the VNC ticket server-side */
  consoleToken?: string;
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
 * @param consoleToken - Optional opaque backend token. When provided, the VNC ticket
 *   is omitted from the URL and the backend resolves the real ticket server-side.
 * @returns The complete WebSocket URL
 * @throws Error if ticket, port, node, or host/protocol are invalid
 */
export function buildWebSocketURL(
  vmid: number,
  ticket: string,
  port: number,
  node: string,
  consoleToken?: string,
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

  // Use the page's own host so the WebSocket routes through the same origin.
  // In dev, this goes through the Vite proxy (ws: true on /api).
  // In production, the SPA is served by the PVMSS backend at the same origin.
  // VITE_BACKEND_HOST / VITE_BACKEND_PROTOCOL are optional overrides for unusual deployments.
  const host =
    import.meta.env.VITE_BACKEND_HOST ||
    (typeof window !== "undefined" ? window.location.host : "");

  if (!host || host.trim().length === 0) {
    throw new Error("Unable to determine backend host.");
  }

  const overrideProtocol = import.meta.env.VITE_BACKEND_PROTOCOL as
    | string
    | undefined;
  if (
    overrideProtocol !== undefined &&
    overrideProtocol !== "ws:" &&
    overrideProtocol !== "wss:"
  ) {
    throw new Error(
      `Invalid WebSocket protocol: ${overrideProtocol}. Must be "ws:" or "wss:"`,
    );
  }
  const protocol =
    overrideProtocol ||
    (typeof window !== "undefined" && window.location.protocol === "https:"
      ? "wss:"
      : "ws:");

  const baseUrl = `${protocol}//${host}/api/v1/vms/${vmid}/console/websocket`;
  const query = buildConsoleQuery({ ticket, port, node, consoleToken });
  return `${baseUrl}?${query}`;
}

/**
 * Builds the query string for the console WebSocket URL.
 *
 * Prefers the opaque backend token (which keeps the VNC ticket out of the
 * browser URL, logs, and proxy access logs). Falls back to the legacy
 * port/node/vncticket triplet when no token is available.
 *
 * If `consoleToken` is defined but empty/whitespace we warn loudly: the
 * caller likely expected the hardened flow and silently falling back to the
 * legacy query would leak the VNC ticket into the URL.
 */
function buildConsoleQuery(params: {
  ticket: string;
  port: number;
  node: string;
  consoleToken?: string;
}): string {
  const { ticket, port, node, consoleToken } = params;
  if (consoleToken !== undefined) {
    const trimmed = consoleToken.trim();
    if (trimmed.length > 0) {
      return `token=${encodeURIComponent(trimmed)}`;
    }
    if (typeof console !== "undefined" && console.warn) {
      console.warn(
        "buildWebSocketURL: consoleToken was provided but empty; falling back to legacy query (VNC ticket will appear in URL).",
      );
    }
  }
  return `port=${port}&node=${encodeURIComponent(node)}&vncticket=${encodeURIComponent(ticket)}`;
}
