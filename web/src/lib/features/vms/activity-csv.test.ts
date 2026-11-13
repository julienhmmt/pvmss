import { describe, expect, it } from 'vitest';
import { activityCsvFilename, buildActivityCsv, buildUserActivityCsv, userActivityCsvFilename } from './activity-csv';
import type { VmAuditEntry } from './detail.svelte';

function entry(overrides: Partial<VmAuditEntry>): VmAuditEntry {
	return {
		id: 1,
		actor: 'alice',
		cluster: 'default',
		vmid: 100,
		action: 'vm.start',
		timestamp: '2026-01-01T00:00:00Z',
		...overrides
	};
}

const label = (action: string): string => `label:${action}`;

describe('buildActivityCsv', () => {
	it('writes the header and one row per entry', () => {
		const csv = buildActivityCsv([entry({})], label);
		expect(csv).toBe('timestamp,actor,action,label\r\n2026-01-01T00:00:00Z,alice,vm.start,label:vm.start\r\n');
	});

	it('writes only the header for an empty list', () => {
		expect(buildActivityCsv([], label)).toBe('timestamp,actor,action,label\r\n');
	});

	it.each([
		['plain value', { actor: 'bob' }, 'bob'],
		['comma', { actor: 'a,b' }, '"a,b"'],
		['double quote', { actor: 'a"b' }, '"a""b"'],
		['newline', { actor: 'a\nb' }, '"a\nb"'],
		['carriage return', { actor: 'a\rb' }, '"a\rb"'],
		['equals sign', { actor: '=cmd' }, "'=cmd"],
		['plus sign', { actor: '+42' }, "'+42"],
		['minus sign', { actor: '-1' }, "'-1"],
		['at sign', { actor: '@sum' }, "'@sum"],
		['formula with comma', { actor: '=a,b' }, '"\'=a,b"'],
		['leading space is not a formula', { actor: ' =x' }, ' =x']
	])('escapes %s', (_name, overrides, expectedCell) => {
		const csv = buildActivityCsv([entry(overrides)], label);
		const row = csv.split('\r\n')[1];
		expect(row).toContain(expectedCell);
	});

	it('escapes the label column too', () => {
		const csv = buildActivityCsv([entry({})], () => '=SUM(1,2)');
		expect(csv.split('\r\n')[1]).toBe('2026-01-01T00:00:00Z,alice,vm.start,"\'=SUM(1,2)"');
	});
});

describe('activityCsvFilename', () => {
	it('formats vm-<vmid>-activity-<YYYYMMDD>.csv', () => {
		const name = activityCsvFilename(100, new Date('2026-01-15T12:30:00Z'));
		expect(name).toBe('vm-100-activity-20260115.csv');
	});
});

describe('buildUserActivityCsv', () => {
	it('adds the machine columns and escapes the name', () => {
		const csv = buildUserActivityCsv(
			[{ id: 'default:1', cluster: 'default', vmid: 100, machineName: '=web', actor: 'alice', action: 'vm.start', timestamp: '2026-01-01T00:00:00Z' }],
			label
		);
		expect(csv).toBe(
			"timestamp,cluster,vmid,machine,actor,action,label\r\n2026-01-01T00:00:00Z,default,100,'=web,alice,vm.start,label:vm.start\r\n"
		);
		expect(userActivityCsvFilename(new Date('2026-01-15T12:30:00Z'))).toBe('activity-20260115.csv');
	});
});
