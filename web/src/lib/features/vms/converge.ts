/**
 * Convergence loop for optimistic VM status (ADR 0001).
 *
 * After a power action POST succeeds, the UI holds the optimistic target
 * status and polls the live-status endpoint until the real state matches.
 * This replaces the old `load()` call that overwrote the optimistic flip
 * with a stale projection read.
 *
 * Used by both the detail store (per-VM endpoint) and the list store
 * (batch endpoint). The loop is extracted here so both stores share one
 * implementation.
 */

import { get, post } from '$lib/shared/api/client';
import type { VmStatus } from './list.svelte';

/** Poll interval for live-status reads during convergence. */
export const ACTION_POLL_MS = 1500;

/** Maximum time to wait for convergence before accepting the last reading. */
export const ACTION_CONVERGE_TIMEOUT_MS = 30_000;

/** Response shape from GET /vms/:cluster/:vmid/status. */
interface LiveStatusResponse {
	status: VmStatus;
	lock?: string;
	uptime: number;
}

/** Response shape from POST /vms/status (batch) — a bare array per ticket 01b. */
type BatchStatusResponse = BatchStatusItem[];

interface BatchStatusItem {
	cluster: string;
	vmid: number;
	status: VmStatus;
	lock?: string;
	uptime: number;
}

/** Target for a single convergence loop. */
export interface ConvergeTarget {
	cluster: string;
	vmid: number;
}

/**
 * Converge toward `targetStatus` by polling the per-VM live-status endpoint.
 *
 * - Polls `GET /api/v1/vms/:cluster/:vmid/status` every `ACTION_POLL_MS`.
 * - Calls `onTick` with each live reading so the store can patch the entity.
 * - Returns when `live.status === targetStatus` (converged) or the timeout
 *   expires (accepts the last reading — the action was accepted by Proxmox,
 *   slow convergence is not a POST failure).
 * - Intermediate read errors are swallowed: the optimistic state survives
 *   and the next tick retries.
 */
export async function convergeSingle(
	target: ConvergeTarget,
	targetStatus: VmStatus,
	onTick: (status: VmStatus) => void,
	signal?: AbortSignal,
): Promise<void> {
	const deadline = Date.now() + ACTION_CONVERGE_TIMEOUT_MS;
	const path = `/api/v1/vms/${encodeURIComponent(target.cluster)}/${target.vmid}/status`;

	while (Date.now() < deadline) {
		if (signal?.aborted) return;

		try {
			const live = await get<LiveStatusResponse>(path);
			onTick(live.status);
			if (live.status === targetStatus) return;
		} catch {
			// Intermediate read error: preserve optimistic state, retry next tick.
		}

		await delay(ACTION_POLL_MS, signal);
	}
}

/**
 * Converge toward `targetStatus` by polling the batch live-status endpoint.
 *
 * - Polls `POST /api/v1/vms/status` with a single target every `ACTION_POLL_MS`.
 * - Calls `onTick` with each live reading so the store can patch the row.
 * - Same convergence and timeout semantics as `convergeSingle`.
 *
 * The batch endpoint is used even for a single row so the list and detail
 * stores share the same code path — no special-casing.
 */
export async function convergeBatch(
	target: ConvergeTarget,
	targetStatus: VmStatus,
	onTick: (status: VmStatus) => void,
	signal?: AbortSignal,
): Promise<void> {
	const deadline = Date.now() + ACTION_CONVERGE_TIMEOUT_MS;

	while (Date.now() < deadline) {
		if (signal?.aborted) return;

		try {
			const resp = await post<BatchStatusResponse>('/api/v1/vms/status', [
				{ cluster: target.cluster, vmid: target.vmid },
			]);
			const item = resp.find(
				(r) => r.cluster === target.cluster && r.vmid === target.vmid,
			);
			if (item) {
				onTick(item.status);
				if (item.status === targetStatus) return;
			}
		} catch {
			// Intermediate read error: preserve optimistic state, retry next tick.
		}

		await delay(ACTION_POLL_MS, signal);
	}
}

/** Resolves after `ms`, or rejects immediately if the signal aborts. */
function delay(ms: number, signal?: AbortSignal): Promise<void> {
	return new Promise((resolve) => {
		if (signal?.aborted) {
			resolve();
			return;
		}
		const timer = setTimeout(resolve, ms);
		signal?.addEventListener(
			'abort',
			() => {
				clearTimeout(timer);
				resolve();
			},
			{ once: true },
		);
	});
}
