import { beforeAll, afterEach, describe, expect, it, vi } from 'vitest';
import { mount, tick } from 'svelte';
import CatalogListStates from './CatalogListStates.svelte';
import { setLocale } from '#lib/paraglide/runtime.js';

const baseProps = {
	loading: false,
	columns: 3,
	totalCount: 1,
	filteredCount: 1,
	loadedLabel: 'loaded',
	emptyTitle: 'empty',
	emptyDescription: 'empty desc',
	emptyActionLabel: 'go',
	noMatchTitle: 'nomatch',
	noMatchDescription: 'nomatch desc',
	resetLabel: 'reset',
	onResetFilters: vi.fn()
};

function render(props: Record<string, unknown>): void {
	mount(CatalogListStates, {
		target: document.body,
		props: { ...baseProps, ...props, children: () => undefined } as never
	});
}

const findButton = (label: string) => [...document.querySelectorAll('button')].find((b) => b.textContent?.trim() === label);

describe('CatalogListStates load error', () => {
	beforeAll(() => setLocale('en', { reload: false }));
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('renders a retry button that calls onRetry once', async () => {
		const onRetry = vi.fn();
		render({ error: 'boom', onRetry });
		await tick();

		const retry = findButton('Retry');
		expect(retry).toBeDefined();
		retry?.click();
		expect(onRetry).toHaveBeenCalledTimes(1);
	});

	it('shows the error without a retry button when onRetry is absent', async () => {
		render({ error: 'boom' });
		await tick();

		expect(document.body.textContent).toContain('boom');
		expect(findButton('Retry')).toBeUndefined();
	});
});
