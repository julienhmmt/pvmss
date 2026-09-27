import { describe, it, expect } from 'vitest';
import layoutSource from './+layout.svelte?raw';

// PVMSS is deployed offline: the shell must not link anywhere except the
// project's GitHub repository.
describe('app shell links', () => {
	it('links to no external host other than GitHub', () => {
		const urls = layoutSource.match(/https?:\/\/[^\s'"`]+/g) ?? [];
		const foreign = urls.filter((url) => !url.startsWith('https://github.com/'));
		expect(foreign).toEqual([]);
	});
});
