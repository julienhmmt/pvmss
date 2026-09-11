import { afterEach, describe, expect, it, vi } from 'vitest';
import { CloudInitFilesStore, type CloudInitFile } from './cloudInitFiles.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

const fullFile: CloudInitFile = {
	id: 'dev-box',
	label: 'Dev box',
	content: '#cloud-config\npackages:\n  - htop\n',
	createdAt: '2026-09-11T10:00:00Z',
	updatedAt: '2026-09-11T10:00:00Z'
};

describe('CloudInitFilesStore', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('loads the file summaries', async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			jsonResponse(200, { files: [{ id: 'dev-box', label: 'Dev box', updatedAt: '2026-09-11T10:00:00Z' }] })
		);
		vi.stubGlobal('fetch', fetchMock);

		const store = new CloudInitFilesStore();
		await store.load();

		expect(fetchMock).toHaveBeenCalledWith('/api/v1/cloudinit/files', expect.anything());
		expect(store.files).toHaveLength(1);
		expect(store.files[0]?.id).toBe('dev-box');
		expect(store.loading).toBe(false);
		expect(store.error).toBeNull();
	});

	it('posts a new file and appends its summary', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(201, fullFile));
		vi.stubGlobal('fetch', fetchMock);

		const store = new CloudInitFilesStore();
		await store.create('Dev box', fullFile.content);

		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/cloudinit/files',
			expect.objectContaining({ method: 'POST' })
		);
		expect(store.files[0]).toEqual({ id: 'dev-box', label: 'Dev box', updatedAt: '2026-09-11T10:00:00Z' });
	});

	it('puts updates and patches the summary row', async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			jsonResponse(200, { ...fullFile, label: 'Dev box v2', updatedAt: '2026-09-11T12:00:00Z' })
		);
		vi.stubGlobal('fetch', fetchMock);

		const store = new CloudInitFilesStore();
		store.files = [{ id: 'dev-box', label: 'Dev box', updatedAt: '2026-09-11T10:00:00Z' }];
		await store.update('dev-box', 'Dev box v2', fullFile.content);

		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/cloudinit/files/dev-box',
			expect.objectContaining({ method: 'PUT' })
		);
		expect(store.files[0]?.label).toBe('Dev box v2');
	});

	it('deletes and drops the row', async () => {
		const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
		vi.stubGlobal('fetch', fetchMock);

		const store = new CloudInitFilesStore();
		store.files = [{ id: 'dev-box', label: 'Dev box', updatedAt: '2026-09-11T10:00:00Z' }];
		await store.remove('dev-box');

		expect(fetchMock).toHaveBeenCalledWith(
			'/api/v1/cloudinit/files/dev-box',
			expect.objectContaining({ method: 'DELETE' })
		);
		expect(store.files).toHaveLength(0);
	});

	it.each([
		['duplicate_cloudinit_file', 'cloudinit.files.errorDuplicate'],
		['cloudinit_file_limit', 'cloudinit.files.errorLimit'],
		['invalid_cloudinit_file', 'cloudinit.files.errorInvalid']
	])('maps %s to a localized save error', async (code, _key) => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(409, { code, message: 'server detail' }));
		vi.stubGlobal('fetch', fetchMock);

		const store = new CloudInitFilesStore();
		await expect(store.create('x', '#cloud-config\n')).rejects.toThrow();

		expect(store.saveError).toBeTruthy();
		expect(store.saveError).not.toBe('server detail');
	});

	it('surfaces load failures on error', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(500, { code: 'internal_error', message: 'boom' }));
		vi.stubGlobal('fetch', fetchMock);

		const store = new CloudInitFilesStore();
		await store.load();

		expect(store.error).toBe('boom');
		expect(store.files).toHaveLength(0);
	});
});
