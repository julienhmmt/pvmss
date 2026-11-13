import { describe, expect, it } from 'vitest';
import { clusterTabParam, parseClusterTab } from './cluster-detail.js';

describe('parseClusterTab', () => {
	it.each([
		['status', 'status'],
		['infrastructure', 'infrastructure'],
		['connection', 'connection'],
		['bogus', 'status'],
		['', 'status'],
		[null, 'status']
	] as const)('maps %s to %s', (raw, want) => {
		expect(parseClusterTab(raw)).toBe(want);
	});
});

describe('clusterTabParam', () => {
	it.each([
		['status', null],
		['infrastructure', 'infrastructure'],
		['connection', 'connection']
	] as const)('serializes %s to %s', (tab, want) => {
		expect(clusterTabParam(tab)).toBe(want);
	});
});
