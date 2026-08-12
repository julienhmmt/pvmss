import { ApiRequestError } from '$lib/shared/api/client';
import { setContext, getContext } from 'svelte';

export interface TablePreview {
	name: string;
	rowCount: number;
}

export interface ImportPreview {
	stagingToken: string;
	expiresAt: string;
	tables: TablePreview[];
	ignoredTables: string[];
}

export interface ImportResult {
	status: string;
	tables: TablePreview[];
}

/**
 * DbOpsStore manages the admin database export/import flow. Export is a
 * simple download trigger; import is a two-step upload → preview → confirm
 * flow, with $state for the in-progress upload/preview (constitution VII).
 */
export class DbOpsStore {
	exporting = $state.raw(false);
	exportError = $state.raw<string | null>(null);

	importing = $state.raw(false);
	importError = $state.raw<string | null>(null);
	preview = $state.raw<ImportPreview | null>(null);

	confirming = $state.raw(false);
	confirmError = $state.raw<string | null>(null);
	confirmResult = $state.raw<ImportResult | null>(null);

	async exportDatabase(): Promise<void> {
		this.exporting = true;
		this.exportError = null;
		try {
			const response = await fetch('/api/v1/admin/db/export');
			if (!response.ok) {
				throw new Error(`export failed: ${response.status}`);
			}
			const blob = await response.blob();
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			const disposition = response.headers.get('Content-Disposition') ?? '';
			const match = disposition.match(/filename="([^"]+)"/);
			a.download = match?.[1] ?? 'pvmss-export.db';
			document.body.appendChild(a);
			a.click();
			document.body.removeChild(a);
			URL.revokeObjectURL(url);
		} catch (err) {
			this.exportError = err instanceof Error ? err.message : 'export failed';
		} finally {
			this.exporting = false;
		}
	}

	async uploadImport(file: File): Promise<void> {
		this.importing = true;
		this.importError = null;
		this.preview = null;
		this.confirmResult = null;
		try {
			const formData = new FormData();
			formData.append('file', file);
			const response = await fetch('/api/v1/admin/db/import', {
				method: 'POST',
				body: formData
			});
			if (!response.ok) {
				let message = 'import failed';
				try {
					const body = await response.json();
					message = body.message ?? message;
				} catch { /* keep default */ }
				throw new ApiRequestError(response.status, 'import_error', message);
			}
			this.preview = (await response.json()) as ImportPreview;
		} catch (err) {
			this.importError = err instanceof Error ? err.message : 'import failed';
		} finally {
			this.importing = false;
		}
	}

	async confirmImport(): Promise<void> {
		if (!this.preview) return;
		this.confirming = true;
		this.confirmError = null;
		try {
			const response = await fetch('/api/v1/admin/db/import/confirm', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ stagingToken: this.preview.stagingToken })
			});
			if (!response.ok) {
				let message = 'confirm failed';
				try {
					const body = await response.json();
					message = body.message ?? message;
				} catch { /* keep default */ }
				throw new ApiRequestError(response.status, 'confirm_error', message);
			}
			this.confirmResult = (await response.json()) as ImportResult;
			this.preview = null;
		} catch (err) {
			this.confirmError = err instanceof Error ? err.message : 'confirm failed';
		} finally {
			this.confirming = false;
		}
	}

	cancelPreview(): void {
		this.preview = null;
		this.confirmResult = null;
		this.importError = null;
		this.confirmError = null;
	}
}

const DB_OPS_CONTEXT_KEY = Symbol('admin-db-ops');

export function setDbOpsContext(): DbOpsStore {
	const store = new DbOpsStore();
	setContext(DB_OPS_CONTEXT_KEY, store);
	return store;
}

export function getDbOpsContext(): DbOpsStore {
	return getContext<DbOpsStore>(DB_OPS_CONTEXT_KEY);
}
