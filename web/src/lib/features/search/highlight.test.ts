import { describe, it, expect } from 'vitest';
import { highlightText, vmidMatches } from './highlight';

describe('highlightText', () => {
	it.each([
		{ text: 'web-01', query: '', expected: [{ text: 'web-01', match: false }] },
		{ text: 'web-01', query: '   ', expected: [{ text: 'web-01', match: false }] },
		{
			text: 'web-01',
			query: 'db',
			expected: [{ text: 'web-01', match: false }]
		},
		{
			text: 'web-01',
			query: 'web',
			expected: [
				{ text: 'web', match: true },
				{ text: '-01', match: false }
			]
		},
		{
			text: 'WEB-01',
			query: 'web',
			expected: [
				{ text: 'WEB', match: true },
				{ text: '-01', match: false }
			]
		},
		{
			text: 'aba',
			query: 'a',
			expected: [
				{ text: 'a', match: true },
				{ text: 'b', match: false },
				{ text: 'a', match: true }
			]
		},
		{
			text: 'testssojhzitadel',
			query: 'testsso',
			expected: [
				{ text: 'testsso', match: true },
				{ text: 'jhzitadel', match: false }
			]
		}
	])('splits "$text" on "$query"', ({ text, query, expected }) => {
		expect(highlightText(text, query)).toEqual(expected);
	});
});

describe('vmidMatches', () => {
	it.each([
		{ vmid: 2222001, query: '2222001', expected: true },
		{ vmid: 2222001, query: ' 2222001 ', expected: true },
		{ vmid: 2222001, query: '2222', expected: false },
		{ vmid: 2222001, query: 'web', expected: false },
		{ vmid: 2222001, query: '', expected: false }
	])('vmid $vmid vs query "$query" -> $expected', ({ vmid, query, expected }) => {
		expect(vmidMatches(vmid, query)).toBe(expected);
	});
});
