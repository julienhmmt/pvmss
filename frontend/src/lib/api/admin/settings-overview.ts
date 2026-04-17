/**
 * API client for settings overview endpoints
 * Phase 9: Unified Settings Panel
 */

export interface SectionMeta {
	name: string;
	category: string;
	kind: string;
	row_count: number;
	last_change_at?: string;
	last_change_by?: string;
	supports_add: boolean;
	supports_edit: boolean;
}

export interface OverviewSection extends SectionMeta {
	data: unknown;
}

export interface OverviewResponse {
	schema_version: number;
	bootstrap_complete: boolean;
	sections: Record<string, OverviewSection>;
}

export interface UpsertRequest {
	table: string;
	record: unknown;
}

export interface UpsertResponse {
	success: boolean;
	message: string;
}

// Category constants
export const CATEGORY_RESOURCES = 'resources';
export const CATEGORY_INVENTORY = 'inventory';
export const CATEGORY_TEMPLATES = 'templates';
export const CATEGORY_INTEGRATIONS = 'integrations';

// Kind constants
export const KIND_SINGLETON = 'singleton';
export const KIND_LIST = 'list';
export const KIND_KEYED = 'keyed';

// Table name constants
export const TABLE_VM_LIMITS = 'vm_limits';
export const TABLE_NODE_LIMITS = 'node_limits';
export const TABLE_ENABLED_NODES = 'enabled_nodes';
export const TABLE_ENABLED_STORAGES = 'enabled_storages';
export const TABLE_ENABLED_ISOS = 'enabled_isos';
export const TABLE_ENABLED_VMBRS = 'enabled_vmbrs';
export const TABLE_TAGS = 'tags';
export const TABLE_CLOUDINIT_TEMPLATES = 'cloudinit_templates';
export const TABLE_VM_PROFILES = 'vm_profiles';
export const TABLE_SFTP_CONFIG = 'sftp_config';

/**
 * Fetch the settings overview
 * GET /api/v1/admin/settings/overview
 */
export async function getSettingsOverview(): Promise<OverviewResponse> {
	const res = await fetch('/api/v1/admin/settings/overview', {
		credentials: 'same-origin',
	});

	if (!res.ok) {
		const err = await res.json().catch(() => ({ message: res.statusText }));
		throw new Error(err.message ?? res.statusText);
	}

	return res.json();
}

/**
 * Upsert settings record
 * POST /api/v1/admin/settings/upsert
 */
export async function upsertSettings(request: UpsertRequest): Promise<UpsertResponse> {
	const res = await fetch('/api/v1/admin/settings/upsert', {
		method: 'POST',
		credentials: 'same-origin',
		headers: {
			'Content-Type': 'application/json',
		},
		body: JSON.stringify(request),
	});

	if (!res.ok) {
		const err = await res.json().catch(() => ({ message: res.statusText }));
		throw new Error(err.message ?? res.statusText);
	}

	return res.json();
}
