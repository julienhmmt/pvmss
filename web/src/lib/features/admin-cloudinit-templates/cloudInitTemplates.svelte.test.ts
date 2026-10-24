import { afterEach, describe, expect, it, vi } from 'vitest';
import { AdminCloudInitTemplatesStore } from './cloudInitTemplates.svelte';
import { m } from '#lib/paraglide/messages.js';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

const template = { id: 'tpl-1', label: 'Base', content: '#cloud-config\n', enabled: true, document: null };

describe('AdminCloudInitTemplatesStore save contract', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('create() returns true and appends the template on success', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(201, template)));
		const store = new AdminCloudInitTemplatesStore();
		await expect(store.create('Base', '#cloud-config\n')).resolves.toBe(true);
		expect(store.templates).toEqual([template]);
		expect(store.saveError).toBeNull();
	});

	it('create() returns false and sets saveError on failure instead of throwing', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(400, { code: 'invalid_yaml', message: 'bad yaml' })));
		const store = new AdminCloudInitTemplatesStore();
		await expect(store.create('Base', 'not yaml: [')).resolves.toBe(false);
		expect(store.templates).toHaveLength(0);
		expect(store.saveError).toBeTruthy();
	});

	it('update() returns false and sets saveError on failure instead of throwing', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(500, { code: 'oops', message: 'server broke' })));
		const store = new AdminCloudInitTemplatesStore();
		store.templates = [template];
		await expect(store.update('tpl-1', 'Base', '#cloud-config\nusers: []')).resolves.toBe(false);
		expect(store.saveError).toBe(m['error.generic']());
		expect(store.templates[0]?.content).toBe('#cloud-config\n');
	});

	it('update() returns true and replaces the template on success', async () => {
		const updated = { ...template, label: 'Base v2' };
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, updated)));
		const store = new AdminCloudInitTemplatesStore();
		store.templates = [template];
		await expect(store.update('tpl-1', 'Base v2', 'x')).resolves.toBe(true);
		expect(store.templates[0]?.label).toBe('Base v2');
	});
});
