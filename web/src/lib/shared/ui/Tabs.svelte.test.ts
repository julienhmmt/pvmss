import { describe, it, expect, afterEach } from 'vitest';
import { flushSync, mount } from 'svelte';
import Tabs from './Tabs.svelte';

const TABS = [
	{ id: 'one', label: () => 'One' },
	{ id: 'two', label: () => 'Two' },
	{ id: 'three', label: () => 'Three' }
];

function press(id: string, key: string): void {
	document.getElementById(`tab-${id}`)?.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
	flushSync();
}

const selected = (): string | undefined => document.querySelector('[role="tab"][aria-selected="true"]')?.id;

describe('Tabs keyboard', () => {
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it.each([
		['two', 'ArrowRight', 'tab-three'],
		['three', 'ArrowRight', 'tab-one'],
		['one', 'ArrowLeft', 'tab-three'],
		['two', 'Home', 'tab-one'],
		['two', 'End', 'tab-three']
	])('from %s, %s selects and focuses %s', (from, key, expected) => {
		mount(Tabs, { target: document.body, props: { tabs: TABS, active: from } });

		press(from, key);
		expect(selected()).toBe(expected);
		expect(document.activeElement?.id).toBe(expected);
	});

	it('keeps only the active tab in the tab order', () => {
		mount(Tabs, { target: document.body, props: { tabs: TABS, active: 'two' } });

		press('two', 'End');
		const order = Array.from(document.querySelectorAll<HTMLElement>('[role="tab"]')).map((tab) => tab.tabIndex);
		expect(order).toEqual([-1, -1, 0]);
	});

	it('ignores other keys', () => {
		mount(Tabs, { target: document.body, props: { tabs: TABS, active: 'two' } });

		press('two', 'a');
		expect(selected()).toBe('tab-two');
	});
});
