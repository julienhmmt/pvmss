import { beforeAll, describe, expect, it } from 'vitest';
import { setLocale } from '#lib/paraglide/runtime.js';
import { buildProfileDraftJson, parseProfileDraftJson } from './profiles.svelte';

const valid = {
	label: 'Expert',
	sockets: 2,
	cpuCores: 4,
	memoryMB: 8192,
	diskGB: 80,
	bus: 'virtio',
	description: 'For build workers'
};

describe('parseProfileDraftJson', () => {
	beforeAll(() => setLocale('en', { reload: false }));

	it('parses a complete profile object', () => {
		expect(parseProfileDraftJson(JSON.stringify(valid))).toEqual(valid);
	});

	it('accepts an omitted description', () => {
		const withoutDescription: Partial<typeof valid> = { ...valid };
		delete withoutDescription.description;
		expect(parseProfileDraftJson(JSON.stringify(withoutDescription))).toEqual({ ...valid, description: '' });
	});

	it('trims the label and the description', () => {
		const draft = parseProfileDraftJson(JSON.stringify({ ...valid, label: '  Expert  ', description: '  note  ' }));
		expect(draft.label).toBe('Expert');
		expect(draft.description).toBe('note');
	});

	it('rejects a value that is not a JSON object', () => {
		expect(() => parseProfileDraftJson('[]')).toThrow(/one profile/i);
	});

	it('reports malformed JSON', () => {
		expect(() => parseProfileDraftJson('{')).toThrow(/invalid json/i);
	});

	it('rejects unknown fields so typos do not save silently', () => {
		expect(() => parseProfileDraftJson(JSON.stringify({ ...valid, cpuCore: 4 }))).toThrow(/unknown field.*cpuCore/i);
	});

	it('rejects a missing required field', () => {
		const withoutBus: Partial<typeof valid> = { ...valid };
		delete withoutBus.bus;
		expect(() => parseProfileDraftJson(JSON.stringify(withoutBus))).toThrow(/missing field.*bus/i);
	});

	it('enforces the server sizing rules', () => {
		expect(() => parseProfileDraftJson(JSON.stringify({ ...valid, cpuCores: 0 }))).toThrow(/cpuCores/i);
		expect(() => parseProfileDraftJson(JSON.stringify({ ...valid, memoryMB: 127 }))).toThrow(/memoryMB/i);
		expect(() => parseProfileDraftJson(JSON.stringify({ ...valid, diskGB: 0 }))).toThrow(/diskGB/i);
		expect(() => parseProfileDraftJson(JSON.stringify({ ...valid, sockets: 0 }))).toThrow(/sockets/i);
	});

	it('rejects a bus outside the form choices', () => {
		expect(() => parseProfileDraftJson(JSON.stringify({ ...valid, bus: 'usb' }))).toThrow(/bus/i);
	});

	it('rejects a description over 500 characters', () => {
		expect(() => parseProfileDraftJson(JSON.stringify({ ...valid, description: 'x'.repeat(501) }))).toThrow(/500/i);
	});
});

describe('buildProfileDraftJson', () => {
	it('emits the editor template for the current values', () => {
		const json = buildProfileDraftJson(valid);
		expect(JSON.parse(json)).toEqual(valid);
	});
});
