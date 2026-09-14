import { describe, it, expect, afterEach } from 'vitest';
import { mount } from 'svelte';
import Select from './Select.svelte';

describe('Select optgroups', () => {
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('renders ungrouped options exactly as before', () => {
		mount(Select, {
			target: document.body,
			props: { value: '', options: ['a', 'b', { value: 'c', label: 'C' }] }
		});

		expect(document.querySelectorAll('optgroup')).toHaveLength(0);
		expect([...document.querySelectorAll('option')].map((o) => o.value)).toEqual(['a', 'b', 'c']);
	});

	it('renders one optgroup per distinct group, ungrouped first', () => {
		mount(Select, {
			target: document.body,
			props: {
				value: '',
				options: [
					{ value: 'u1', label: 'Ungrouped' },
					{ value: 'a1', label: 'Admin one', group: 'Admin' },
					{ value: 'm1', label: 'Mine one', group: 'Mine' },
					{ value: 'a2', label: 'Admin two', group: 'Admin' }
				]
			}
		});

		const groups = [...document.querySelectorAll('optgroup')];
		expect(groups.map((g) => g.label)).toEqual(['Admin', 'Mine']);
		expect([...groups[0]!.querySelectorAll('option')].map((o) => o.value)).toEqual(['a1', 'a2']);
		expect([...groups[1]!.querySelectorAll('option')].map((o) => o.value)).toEqual(['m1']);
		// The ungrouped option is a direct child of the select, before the groups.
		expect(document.querySelector<HTMLOptionElement>('select > option')?.value).toBe('u1');
	});
});
