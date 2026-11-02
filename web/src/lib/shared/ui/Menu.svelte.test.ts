import { describe, it, expect, afterEach, vi } from 'vitest';
import { mount, tick, unmount, type Component } from 'svelte';
import Menu from './Menu.svelte';

const menu = (): HTMLElement | null => document.body.querySelector<HTMLElement>('[role="menu"]');
const trigger = (): HTMLElement => document.body.querySelector<HTMLElement>('[data-testid="m"]')!;
const items = (): HTMLElement[] => [...document.body.querySelectorAll<HTMLElement>('[role="menuitem"]')];

describe('Menu', () => {
	let mounted: Record<string, unknown> | undefined;

	afterEach(async () => {
		if (mounted) await unmount(mounted);
		mounted = undefined;
		vi.useRealTimers();
		document.body.innerHTML = '';
	});

	function open(): void {
		trigger().dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0 }));
	}

	it('opens on the trigger and lists its items', async () => {
		mounted = mount(Menu as unknown as Component, {
			target: document.body,
			props: { label: 'More', testId: 'm', items: [{ label: 'Rename', onclick: () => {} }] }
		});
		await tick();

		expect(trigger().getAttribute('aria-label')).toBe('More');
		open();
		await tick();

		expect(menu()).not.toBeNull();
		expect(items().map((i) => i.textContent?.trim())).toEqual(['Rename']);
	});

	it('fires the item callback on select', async () => {
		const onclick = vi.fn();
		mounted = mount(Menu as unknown as Component, {
			target: document.body,
			props: {
				label: 'More',
				testId: 'm',
				items: [
					{ label: 'Restart', onclick },
					{ label: 'Delete', onclick: () => {}, destructive: true }
				]
			}
		});
		await tick();
		open();
		await tick();

		items()[0]!.dispatchEvent(new PointerEvent('click', { bubbles: true }));
		await tick();

		expect(onclick).toHaveBeenCalledOnce();
	});

	// Escape, focus return and select-to-close are bits-ui/dismissible-layer
	// machinery that happy-dom cannot exercise; they are asserted in the e2e
	// pass on the compact VM action bar (ticket 04's seam).
});
