import { describe, it, expect } from 'vitest';
import { TaskOutcomeLedger } from './task-outcome-ledger.svelte';

describe('TaskOutcomeLedger', () => {
	it('records and reads outcomes by cluster + vmid', () => {
		const ledger = new TaskOutcomeLedger();
		ledger.record('default', 100, 'partial');
		expect(ledger.get('default', 100)).toBe('partial');
	});

	it('overwrites a previous outcome for the same VM', () => {
		const ledger = new TaskOutcomeLedger();
		ledger.record('default', 100, 'partial');
		ledger.record('default', 100, 'failed');
		expect(ledger.get('default', 100)).toBe('failed');
	});

	it('returns undefined for an unrecorded VM', () => {
		const ledger = new TaskOutcomeLedger();
		expect(ledger.get('default', 999)).toBeUndefined();
	});

	it('clears a recorded outcome (idempotent)', () => {
		const ledger = new TaskOutcomeLedger();
		ledger.record('default', 100, 'partial');
		ledger.clear('default', 100);
		expect(ledger.get('default', 100)).toBeUndefined();
		// clearing a missing entry is a no-op
		ledger.clear('default', 100);
		expect(ledger.get('default', 100)).toBeUndefined();
	});

	it('isolates outcomes by cluster', () => {
		const ledger = new TaskOutcomeLedger();
		ledger.record('default', 100, 'partial');
		ledger.record('staging', 100, 'failed');
		expect(ledger.get('default', 100)).toBe('partial');
		expect(ledger.get('staging', 100)).toBe('failed');
	});
});
