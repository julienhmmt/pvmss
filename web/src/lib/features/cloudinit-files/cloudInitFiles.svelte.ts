import { get, post, put, del, ApiRequestError } from '$lib/shared/api/client';
import { setContext, getContext } from 'svelte';
import { m } from '$lib/paraglide/messages.js';

/** List rows carry no content — GET /files/{id} returns the full document. */
export interface CloudInitFileSummary {
	id: string;
	label: string;
	updatedAt: string;
}

export interface CloudInitFile extends CloudInitFileSummary {
	content: string;
	createdAt: string;
}

interface CloudInitFileList {
	files: CloudInitFileSummary[];
}

/** Maps a domain error code to its localized message; anything else falls
 *  back to the API's own message. */
function fileSaveError(err: unknown): string {
	if (err instanceof ApiRequestError) {
		switch (err.code) {
			case 'duplicate_cloudinit_file':
				return m['cloudinit.files.errorDuplicate']();
			case 'cloudinit_file_limit':
				return m['cloudinit.files.errorLimit']();
			case 'invalid_cloudinit_file':
				return m['cloudinit.files.errorInvalid']();
			default:
				return err.message;
		}
	}
	return m['cloudinit.files.loadError']();
}

/**
 * CloudInitFilesStore manages the signed-in user's own cloud-init documents
 * (no cluster, no enable flag — every authenticated user has a private set).
 * API responses are $state.raw — they are API data, not form edits.
 */
export class CloudInitFilesStore {
	files = $state.raw<CloudInitFileSummary[]>([]);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	saving = $state.raw(false);
	saveError = $state.raw<string | null>(null);

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			const result = await get<CloudInitFileList>('/api/v1/cloudinit/files');
			this.files = result.files;
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['cloudinit.files.loadError']();
		} finally {
			this.loading = false;
		}
	}

	/** Fetches one file's full content for the edit dialog. Returns null and
	 *  sets saveError on failure so the caller can leave the dialog closed. */
	async getFile(id: string): Promise<CloudInitFile | null> {
		this.saveError = null;
		try {
			return await get<CloudInitFile>(`/api/v1/cloudinit/files/${id}`);
		} catch (err) {
			this.saveError = fileSaveError(err);
			return null;
		}
	}

	async create(label: string, content: string): Promise<void> {
		this.saving = true;
		this.saveError = null;
		try {
			const created = await post<CloudInitFile>('/api/v1/cloudinit/files', { label, content });
			this.files = [...this.files, { id: created.id, label: created.label, updatedAt: created.updatedAt }];
		} catch (err) {
			this.saveError = fileSaveError(err);
			throw err;
		} finally {
			this.saving = false;
		}
	}

	async update(id: string, label: string, content: string): Promise<void> {
		this.saving = true;
		this.saveError = null;
		try {
			const updated = await put<CloudInitFile>(`/api/v1/cloudinit/files/${id}`, { label, content });
			this.files = this.files.map((f) =>
				f.id === id ? { id: updated.id, label: updated.label, updatedAt: updated.updatedAt } : f
			);
		} catch (err) {
			this.saveError = fileSaveError(err);
			throw err;
		} finally {
			this.saving = false;
		}
	}

	async remove(id: string): Promise<void> {
		this.saveError = null;
		try {
			await del<void>(`/api/v1/cloudinit/files/${id}`);
			this.files = this.files.filter((f) => f.id !== id);
		} catch (err) {
			this.saveError = fileSaveError(err);
			throw err;
		}
	}
}

const CLOUDINIT_FILES_CONTEXT_KEY = Symbol('cloudinit-files');

export function setCloudInitFilesContext(): CloudInitFilesStore {
	const store = new CloudInitFilesStore();
	setContext(CLOUDINIT_FILES_CONTEXT_KEY, store);
	return store;
}

export function getCloudInitFilesContext(): CloudInitFilesStore {
	return getContext<CloudInitFilesStore>(CLOUDINIT_FILES_CONTEXT_KEY);
}
