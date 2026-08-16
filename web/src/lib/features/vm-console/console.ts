import { base } from '$app/paths';
import { post, ApiRequestError } from '$lib/shared/api/client';

/** VNC ticket response from POST /api/v1/vms/:cluster/:vmid/vnc-ticket. */
export interface VncTicketResponse {
	token: string;
	expiresInSeconds: number;
}

/**
 * Requests a single-use console ticket from the backend. The ticket is an
 * opaque token — no Proxmox ticket, node, or port leaks to the client
 * (FR-002, FR-003). The token is consumed exactly once when the WebSocket
 * opens (FR-004).
 *
 * Throws ApiRequestError on non-2xx (403 for non-owners, 404 for unknown VMs,
 * 401 for unauthenticated, 503 if the inventory is not ready).
 */
export async function fetchConsoleTicket(cluster: string, vmid: number): Promise<string> {
	const path = `${base}/api/v1/vms/${encodeURIComponent(cluster)}/${vmid}/vnc-ticket`;
	const response = await post<VncTicketResponse>(path);
	return response.token;
}

/**
 * Builds the WebSocket URL for the console relay endpoint. The opaque token
 * travels as a query parameter — it is single-use and short-TTL, so it is not
 * a standing capability (FR-004). The URL is always same-origin: the browser
 * connects to the same host that served the page, and the backend relays to
 * the cluster's VNC server (FR-007).
 *
 * The scheme is derived from the page protocol: wss: for https:, ws: for
 * http:. This matches the legacy buildWebSocketURL behavior (F07) without the
 * VITE_BACKEND_HOST / VITE_BACKEND_PROTOCOL overrides — same-origin only.
 */
export function buildConsoleWebSocketURL(cluster: string, vmid: number, token: string): string {
	const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
	const host = window.location.host;
	const path = `${base}/api/v1/vms/${encodeURIComponent(cluster)}/${vmid}/console/websocket`;
	return `${protocol}//${host}${path}?token=${encodeURIComponent(token)}`;
}

/** Maps an ApiRequestError to a user-facing message. */
export function consoleTicketErrorMessage(err: unknown, fallback: () => string): string {
	return err instanceof ApiRequestError ? err.message : fallback();
}
