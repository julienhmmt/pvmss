import type { VmQuota } from '$lib/features/vms/list.svelte';

/**
 * quotaMeterView — pure presentation projection for a quota / usage meter.
 *
 * Contract (specs/002-design-import/data-model.md):
 *   - `allowed === -1`  → unlimited: no fake 0–100 bar, no numeric denominator.
 *   - `used >= allowed` (allowed >= 0) → exhausted, clamped to 100%.
 *   - fetch failure (null / undefined) → unavailable, never a misleading "0 / 0".
 *
 * `bounded` is true only when a `role="meter"` bar is meaningful (bounded or
 * exhausted). Unlimited and unavailable render text only — no fake bar.
 */
export type QuotaMeterState = 'bounded' | 'unlimited' | 'exhausted' | 'unavailable';

export interface QuotaMeterView {
	state: QuotaMeterState;
	used: number | null;
	allowed: number | null;
	percent: number | null;
	bounded: boolean;
}

const UNLIMITED_ALLOWED = -1;

export function quotaMeterView(quota: VmQuota | null | undefined): QuotaMeterView {
	if (quota === null || quota === undefined) {
		return { state: 'unavailable', used: null, allowed: null, percent: null, bounded: false };
	}
	if (quota.allowed === UNLIMITED_ALLOWED) {
		// Unlimited: never treat -1 as a numeric denominator. No fake 0–100 bar.
		return { state: 'unlimited', used: quota.used, allowed: null, percent: null, bounded: false };
	}
	const raw = quota.allowed > 0 ? (quota.used / quota.allowed) * 100 : 0;
	const percent = Math.min(100, Math.max(0, Math.round(raw)));
	if (quota.used >= quota.allowed) {
		return { state: 'exhausted', used: quota.used, allowed: quota.allowed, percent: 100, bounded: true };
	}
	return { state: 'bounded', used: quota.used, allowed: quota.allowed, percent, bounded: true };
}
