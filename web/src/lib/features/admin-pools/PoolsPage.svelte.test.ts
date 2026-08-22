import { describe, it, expect, vi } from 'vitest';
import { mount, tick } from 'svelte';
import PoolsPage from './PoolsPage.svelte';
import type { AdminPool } from './pools.svelte';

const noop = () => undefined;

function buildProps(pools: AdminPool[], onSearch = noop) {
	return {
		target: document.body,
		props: {
			pools,
			loading: false,
			error: null,
			saving: false,
			saveError: null,
			deleting: null,
			deleteError: null,
			announce: null,
			credentials: null,
			onSearch,
			onCreate: async () => {},
			onDelete: async () => {},
			onDismissCredentials: noop
		}
	};
}

function pool(name: string, managed: boolean, total = 0, running = 0, stopped = 0): AdminPool {
	return { name, comment: '', total, running, stopped, managed };
}

describe('PoolsPage', () => {
	it.each([
		{
			pools: [pool('pvmss-pool', true), pool('proxmox-pool', false)],
			visible: ['pvmss-pool'],
			hidden: ['proxmox-pool']
		},
		{ pools: [pool('one', true), pool('two', true)], visible: ['one', 'two'], hidden: [] },
		{ pools: [pool('proxmox-only', false)], visible: [], hidden: ['proxmox-only'] }
	])('renders only PVMSS-managed pools ($pools)', ({ pools, visible, hidden }) => {
		mount(PoolsPage, buildProps(pools));
		const text = document.body.textContent ?? '';
		for (const name of visible) expect(text).toContain(name);
		for (const name of hidden) expect(text).not.toContain(name);
		document.body.innerHTML = '';
	});

	it('sorts by name ascending by default', () => {
		mount(PoolsPage, buildProps([pool('zebra', true), pool('alpha', true)]));
		const first = document.querySelector('tbody tr')?.textContent ?? '';
		expect(first).toContain('alpha');
		expect(first).not.toContain('zebra');
		document.body.innerHTML = '';
	});

	it('sorts by total when the total header is clicked', async () => {
		mount(PoolsPage, buildProps([pool('large', true, 5), pool('small', true, 1)]));

		const totalButton = Array.from(document.querySelectorAll('th button')).find((button) =>
			button.textContent?.includes('Total')
		);
		expect(totalButton).toBeDefined();

		totalButton?.click();
		await tick();

		const asc = document.querySelector('tbody tr')?.textContent ?? '';
		expect(asc).toContain('small');
		expect(asc).not.toContain('large');

		totalButton?.click();
		await tick();

		const desc = document.querySelector('tbody tr')?.textContent ?? '';
		expect(desc).toContain('large');
		expect(desc).not.toContain('small');

		document.body.innerHTML = '';
	});

	it('calls onSearch when the search input changes', () => {
		const onSearch = vi.fn();
		mount(PoolsPage, buildProps([pool('searchable', true)], onSearch));

		const input = document.querySelector('input[type="search"]') as HTMLInputElement;
		expect(input).not.toBeNull();

		input.value = 'prod';
		input.dispatchEvent(new InputEvent('input', { bubbles: true, data: 'prod' }));

		expect(onSearch).toHaveBeenCalledWith('prod');
		document.body.innerHTML = '';
	});

	it('resets search when the reset button is clicked', async () => {
		const onSearch = vi.fn();
		mount(PoolsPage, buildProps([pool('resettable', true)], onSearch));

		const input = document.querySelector('input[type="search"]') as HTMLInputElement;
		input.value = 'prod';
		input.dispatchEvent(new InputEvent('input', { bubbles: true, data: 'prod' }));
		expect(onSearch).toHaveBeenCalledWith('prod');

		const resetButton = Array.from(document.querySelectorAll('button')).find((button) =>
			button.textContent?.includes('Réinitialiser')
		);
		expect(resetButton).toBeDefined();
		resetButton?.click();
		await tick();

		expect(onSearch).toHaveBeenLastCalledWith('');
		document.body.innerHTML = '';
	});

	it('shows an empty state when no managed pools exist', () => {
		mount(PoolsPage, buildProps([pool('proxmox-only', false)]));
		expect(document.body.textContent).toContain('Aucun pool');
		document.body.innerHTML = '';
	});
});
