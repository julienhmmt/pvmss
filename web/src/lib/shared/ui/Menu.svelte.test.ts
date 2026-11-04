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
		trigger().click();
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

	it('closes on Escape, returns focus and moves with the arrow keys', async () => {
		mounted = mount(Menu as unknown as Component, {
			target: document.body,
			props: {
				label: 'More',
				testId: 'm',
				items: [
					{ label: 'One', onclick: () => {} },
					{ label: 'Off', onclick: () => {}, disabled: true },
					{ label: 'Two', onclick: () => {} }
				]
			}
		});
		await tick();
		open();
		await tick();
		await tick();
		expect(document.activeElement).toBe(items()[0]);

		menu()!.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
		expect(document.activeElement).toBe(items()[2]);

		menu()!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
		await tick();
		expect(menu()).toBeNull();
		expect(document.activeElement).toBe(trigger());
	});

	it('closes after selecting an item', async () => {
		mounted = mount(Menu as unknown as Component, {
			target: document.body,
			props: { label: 'More', testId: 'm', items: [{ label: 'One', onclick: () => {} }] }
		});
		await tick();
		open();
		await tick();
		items()[0]!.click();
		await tick();
		expect(menu()).toBeNull();
	});
});
