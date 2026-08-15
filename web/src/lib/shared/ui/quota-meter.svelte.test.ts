import { describe, expect, it } from 'vitest';
import { quotaMeterView } from './quota-meter.svelte';
import type { VmQuota } from '$lib/features/vms/list.svelte';

// T005 (US1/US2): quotaMeterView — pure presentation math for the sidebar /
// list quota meter. Tested without a DOM. The contract (data-model.md):
//   - allowed === -1  → unlimited (no fake 0–100 bar, no numeric denominator)
//   - used >= allowed (allowed >= 0) → exhausted
//   - fetch failure (no quota) → "unavailable", never a misleading "0 / 0"
// role="meter" applies only when bounded (bounded || exhausted).

describe('quotaMeterView', () => {
	describe('bounded quota', () => {
		it('computes a 0–100 percent from used / allowed', () => {
			const view = quotaMeterView({ used: 2, allowed: 5 } satisfies VmQuota);
			expect(view.state).toBe('bounded');
			expect(view.percent).toBe(40);
			expect(view.used).toBe(2);
			expect(view.allowed).toBe(5);
			expect(view.bounded).toBe(true);
		});

		it('rounds percent to the nearest integer', () => {
			const view = quotaMeterView({ used: 1, allowed: 3 } satisfies VmQuota);
			expect(view.percent).toBe(33);
		});

		it('zero usage is bounded at 0%', () => {
			const view = quotaMeterView({ used: 0, allowed: 5 } satisfies VmQuota);
			expect(view.state).toBe('bounded');
			expect(view.percent).toBe(0);
			expect(view.bounded).toBe(true);
		});
	});

	describe('unlimited quota (allowed === -1)', () => {
		it('reports unlimited with no percent and no numeric denominator', () => {
			const view = quotaMeterView({ used: 2, allowed: -1 } satisfies VmQuota);
			expect(view.state).toBe('unlimited');
			expect(view.percent).toBeNull();
			expect(view.allowed).toBeNull();
			expect(view.bounded).toBe(false);
		});

		// T022: allowed === -1 must never be treated as a numeric denominator.
		it('never divides by -1 (no negative or NaN percent)', () => {
			const view = quotaMeterView({ used: 7, allowed: -1 } satisfies VmQuota);
			expect(view.percent).toBeNull();
			expect(Number.isNaN(view.percent)).toBe(false);
		});
	});

	describe('exhausted quota (used >= allowed, allowed >= 0)', () => {
		it('clamps to 100% when used equals allowed', () => {
			const view = quotaMeterView({ used: 5, allowed: 5 } satisfies VmQuota);
			expect(view.state).toBe('exhausted');
			expect(view.percent).toBe(100);
			expect(view.bounded).toBe(true);
		});

		it('clamps to 100% when used exceeds allowed', () => {
			const view = quotaMeterView({ used: 6, allowed: 5 } satisfies VmQuota);
			expect(view.state).toBe('exhausted');
			expect(view.percent).toBe(100);
		});
	});

	describe('fetch failure (no quota)', () => {
		it('reports unavailable and never renders a 0 / 0 numeric view', () => {
			const view = quotaMeterView(null);
			expect(view.state).toBe('unavailable');
			expect(view.used).toBeNull();
			expect(view.allowed).toBeNull();
			expect(view.percent).toBeNull();
			expect(view.bounded).toBe(false);
		});

		it('treats undefined the same as null', () => {
			const view = quotaMeterView(undefined);
			expect(view.state).toBe('unavailable');
			expect(view.bounded).toBe(false);
		});
	});
});
