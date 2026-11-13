import { describe, it, expect, afterEach, vi } from 'vitest';
import { mount, tick, unmount, type Component } from 'svelte';
import Tooltip from './Tooltip.svelte';

const tooltip = (): HTMLElement | null => document.body.querySelector<HTMLElement>('[role="tooltip"]');
const trigger = (): HTMLElement => document.body.querySelector<HTMLElement>('[data-tooltip-trigger]')!;

async function showOnHover(): Promise<void> {
	await tick();
	trigger().dispatchEvent(new Event('pointerenter'));
	await vi.advanceTimersByTimeAsync(250);
	await tick();
}

describe('Tooltip', () => {
	let mounted: Record<string, unknown> | undefined;

	function mountTooltip(props: { text: string; position?: 'top' | 'bottom' }): void {
		mounted = mount(Tooltip as unknown as Component, { target: document.body, props });
	}

	afterEach(async () => {
		if (mounted) await unmount(mounted);
		mounted = undefined;
		vi.useRealTimers();
		document.body.innerHTML = '';
	});

	it('renders a focusable trigger carrying the children', () => {
		mountTooltip({ text: 'Hint' });

		expect(trigger()).not.toBeNull();
		expect(trigger().tabIndex).toBe(0);
	});

	it('shows the bubble with its text after the hover delay', async () => {
		vi.useFakeTimers();
		mountTooltip({ text: 'Helpful hint' });

		expect(tooltip()).toBeNull();
		await showOnHover();

		expect(tooltip()).not.toBeNull();
		expect(tooltip()!.textContent?.trim()).toBe('Helpful hint');
	});

	it('shows on keyboard focus too', async () => {
		vi.useFakeTimers();
		mountTooltip({ text: 'Hint' });

		await tick();
		trigger().dispatchEvent(new FocusEvent('focusin', { bubbles: true }));
		await vi.advanceTimersByTimeAsync(250);
		await tick();

		expect(tooltip()).not.toBeNull();
	});

	it('interactive mode leaves tabbability to the child', () => {
		mounted = mount(Tooltip as unknown as Component, {
			target: document.body,
			props: { text: 'Hint', interactive: true }
		});

		expect(trigger().tabIndex).toBe(-1);
	});

	it('describes the trigger with the bubble while open', async () => {
		vi.useFakeTimers();
		mountTooltip({ text: 'Hint' });

		await showOnHover();

		const describedBy = trigger().getAttribute('aria-describedby');
		expect(describedBy).not.toBeNull();
		expect(document.getElementById(describedBy!)).toBe(tooltip());
	});

	it('hides when the pointer leaves', async () => {
		vi.useFakeTimers();
		mountTooltip({ text: 'Hint' });

		await showOnHover();
		expect(tooltip()).not.toBeNull();

		trigger().dispatchEvent(new Event('pointerleave'));
		await vi.advanceTimersByTimeAsync(450);
		await tick();

		expect(tooltip()).toBeNull();
	});

	// Near-top-edge flip is asserted in e2e: happy-dom has no real layout.
});
