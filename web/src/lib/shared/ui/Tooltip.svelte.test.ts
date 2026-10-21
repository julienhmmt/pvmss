import { describe, it, expect, afterEach, vi } from 'vitest';
import { mount, tick } from 'svelte';
import Tooltip from './Tooltip.svelte';

const tooltip = (): HTMLElement => document.body.querySelector<HTMLElement>('[role="tooltip"]')!;
const trigger = (): HTMLElement => document.body.querySelector<HTMLElement>('[aria-describedby]')!;

async function hoverAt(left: number, width: number, bubbleWidth: number): Promise<void> {
	vi.spyOn(trigger(), 'getBoundingClientRect').mockReturnValue({ left, width, top: 100, bottom: 120 } as DOMRect);
	vi.spyOn(tooltip(), 'offsetWidth', 'get').mockReturnValue(bubbleWidth);
	trigger().dispatchEvent(new MouseEvent('mouseenter'));
	await vi.advanceTimersByTimeAsync(200);
	await tick();
}

describe('Tooltip', () => {
	afterEach(() => {
		vi.useRealTimers();
		vi.restoreAllMocks();
		document.body.innerHTML = '';
	});

	it('points aria-describedby at a node that exists while hidden', () => {
		mount(Tooltip, { target: document.body, props: { text: 'Helpful hint' } });

		const target = document.getElementById(trigger().getAttribute('aria-describedby')!);
		expect(target).toBe(tooltip());
		expect(tooltip().hidden).toBe(true);
		expect(tooltip().textContent?.trim()).toBe('Helpful hint');
	});

	it('makes the trigger reachable by keyboard and shows on focus', async () => {
		vi.useFakeTimers();
		mount(Tooltip, { target: document.body, props: { text: 'Hint' } });

		expect(trigger().tabIndex).toBe(0);
		trigger().dispatchEvent(new FocusEvent('focusin', { bubbles: true }));
		await vi.advanceTimersByTimeAsync(200);
		expect(tooltip().hidden).toBe(false);
	});

	it('clamps the bubble inside the left edge of the viewport', async () => {
		vi.useFakeTimers();
		mount(Tooltip, { target: document.body, props: { text: 'Hint' } });

		await hoverAt(0, 10, 100);
		// Center 5 would overflow; 8px margin + half the bubble (50).
		expect(tooltip().style.left).toBe('58px');
	});

	it('clamps the bubble inside the right edge of the viewport', async () => {
		vi.useFakeTimers();
		mount(Tooltip, { target: document.body, props: { text: 'Hint' } });

		await hoverAt(window.innerWidth - 10, 10, 100);
		expect(tooltip().style.left).toBe(`${window.innerWidth - 58}px`);
	});

	it('keeps the bubble centered on the trigger when it fits', async () => {
		vi.useFakeTimers();
		mount(Tooltip, { target: document.body, props: { text: 'Hint' } });

		await hoverAt(400, 20, 100);
		expect(tooltip().style.left).toBe('410px');
	});
});
