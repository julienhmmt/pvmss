import { describe, it, expect } from 'vitest';
import { mount } from 'svelte';
import PoolVmBar from './PoolVmBar.svelte';

interface TestCase {
	running: number;
	stopped: number;
	total: number;
	runningPercent: number | null;
}

const cases: TestCase[] = [
	{ running: 0, stopped: 0, total: 0, runningPercent: null },
	{ running: 1, stopped: 1, total: 2, runningPercent: 50 },
	{ running: 2, stopped: 7, total: 9, runningPercent: 22 },
	{ running: 9, stopped: 0, total: 9, runningPercent: 100 }
];

describe('PoolVmBar', () => {
	it.each(cases)('running=$running / total=$total -> aria-valuenow=$runningPercent', ({ running, stopped, total, runningPercent }) => {
		mount(PoolVmBar, { target: document.body, props: { running, stopped, total } });

		const meter = document.querySelector('[role="meter"]');

		if (runningPercent === null) {
			expect(meter).toBeNull();
			expect(document.body.textContent).toContain('—');
		} else {
			expect(meter).not.toBeNull();
			expect(meter?.getAttribute('aria-valuenow')).toBe(String(runningPercent));
		}

		document.body.innerHTML = '';
	});
});
