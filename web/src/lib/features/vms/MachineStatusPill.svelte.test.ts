import { describe, it, expect, afterEach } from 'vitest';
import { mount } from 'svelte';
import MachineStatusPill from './MachineStatusPill.svelte';
import type { MachineDisplayStatus } from './display-status';

describe('MachineStatusPill', () => {
	afterEach(() => {
		document.body.innerHTML = '';
	});

	const cases: { status: MachineDisplayStatus; toneClass: string }[] = [
		{ status: 'running', toneClass: 'bg-success-soft' },
		{ status: 'stopped', toneClass: 'bg-muted' },
		{ status: 'provisioning', toneClass: 'bg-warning-soft' },
		{ status: 'starting', toneClass: 'bg-warning-soft' },
		{ status: 'stopping', toneClass: 'bg-warning-soft' },
		{ status: 'partial', toneClass: 'bg-warning-soft' },
		{ status: 'failed', toneClass: 'bg-destructive-soft' }
	];

	for (const { status, toneClass } of cases) {
		it(`maps ${status} to the ${toneClass} tone`, () => {
			mount(MachineStatusPill, { target: document.body, props: { status } });
			const pill = document.body.querySelector('span.inline-flex');
			expect(pill, `pill should render for ${status}`).not.toBeNull();
			expect(pill!.className, `${status} should use ${toneClass}`).toContain(toneClass);
		});
	}

	it('renders a non-empty label for every status', () => {
		for (const status of cases.map((c) => c.status)) {
			document.body.innerHTML = '';
			mount(MachineStatusPill, { target: document.body, props: { status } });
			const text = document.body.textContent?.trim() ?? '';
			expect(text.length, `${status} label should not be empty`).toBeGreaterThan(0);
		}
	});

	it('forwards pending to the underlying Pill (pulsing dot)', () => {
		mount(MachineStatusPill, {
			target: document.body,
			props: { status: 'starting', pending: true }
		});
		const dot = document.body.querySelector('span.animate-pulse');
		expect(dot).not.toBeNull();
	});
});
