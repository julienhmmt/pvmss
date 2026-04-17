/**
 * Type definitions for admin settings overview
 * Phase 9: Unified Settings Panel
 * Mirrors backend snapshot structure from admin_settings_overview.go
 */

export interface OverviewSection {
	name: string;
	row_count: number;
	updated_at?: string;
	supports_add: boolean;
	supports_edit: boolean;
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

// Specific section data types (when known)
export interface VMLimitsData {
	max_vms: number;
	max_vm_per_user: number;
	max_network_cards: number;
	max_disk_per_vm: number;
	allow_custom_yaml: boolean;
	max_snapshots: number;
}

export interface NodeLimitData {
	[node_name: string]: number;
}

export interface ListData extends Array<string> {}

export interface SFTPConfigData {
	enabled: boolean;
	host?: string;
	port?: number;
	username?: string;
	private_key_path?: string;
	remote_path?: string;
}
