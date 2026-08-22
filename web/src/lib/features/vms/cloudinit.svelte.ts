import { get, post, put, ApiRequestError } from '$lib/shared/api/client';
import { m } from '$lib/paraglide/messages.js';

export type CloudInitIPMode = 'dhcp' | 'static';

export interface CloudInitConfig {
	user: string;
	sshKeys: string[];
	ipMode: CloudInitIPMode;
	ipAddress?: string;
	gateway?: string;
	dnsServer?: string;
	searchDomain?: string;
}

export interface CloudInitConfigUpdate {
	user?: string;
	password?: string;
	sshKeys?: string[];
	ipMode?: CloudInitIPMode;
	ipAddress?: string;
	gateway?: string;
	dnsServer?: string;
	searchDomain?: string;
}

export interface CloudInitSnippet {
	content: string | null;
	updatedAt: string | null;
	updatedBy: string | null;
}

interface CloudInitUpdateResponse {
	status: string;
	rebooted: boolean;
}

interface CloudInitSnippetResponse {
	status: string;
}

export interface CloudInitSSHKeyResponse {
	status: string;
}

export class CloudInitStore {
	readonly cluster: string;
	readonly vmid: number;

	config = $state.raw<CloudInitConfig | null>(null);
	snippet = $state.raw<CloudInitSnippet | null>(null);
	configLoading = $state.raw(false);
	snippetLoading = $state.raw(false);
	configInFlight = $state.raw(false);
	snippetInFlight = $state.raw(false);
	sshKeyInFlight = $state.raw(false);
	configError = $state.raw<string | null>(null);
	snippetError = $state.raw<string | null>(null);
	snippetErrorCode = $state.raw<string | null>(null);
	sshKeyError = $state.raw<string | null>(null);
	sshKeyErrorCode = $state.raw<string | null>(null);

	#basePath: string;
	#reloadVm: () => Promise<void>;

	constructor(cluster: string, vmid: number, reloadVm: () => Promise<void> = async () => {}) {
		this.cluster = cluster;
		this.vmid = vmid;
		this.#basePath = `/api/v1/vms/${encodeURIComponent(cluster)}/${vmid}/cloudinit`;
		this.#reloadVm = reloadVm;
	}

	async loadConfig(): Promise<void> {
		this.configLoading = true;
		this.configError = null;
		try {
			this.config = sanitizeConfig(await get<CloudInitConfig>(this.#basePath));
		} catch (err) {
			this.configError = errorMessage(err, () => m['vms.cloudinit.errorLoadConfig']());
		} finally {
			this.configLoading = false;
		}
	}

	async saveConfig(update: CloudInitConfigUpdate, rebootNow: boolean): Promise<boolean> {
		if (this.configInFlight) return false;
		this.configInFlight = true;
		this.configError = null;
		try {
			await put<CloudInitUpdateResponse>(this.#basePath, { ...update, rebootNow });
			await this.loadConfig();
			await this.#reloadVm();
			return this.configError === null;
		} catch (err) {
			this.configError = errorMessage(err, () => m['vms.cloudinit.errorSaveConfig']());
			return false;
		} finally {
			this.configInFlight = false;
		}
	}

	async loadSnippet(): Promise<void> {
		this.snippetLoading = true;
		this.snippetError = null;
		this.snippetErrorCode = null;
		try {
			this.snippet = await get<CloudInitSnippet>(`${this.#basePath}/snippet`);
		} catch (err) {
			this.snippetError = errorMessage(err, () => m['vms.cloudinit.errorLoadSnippet']());
		} finally {
			this.snippetLoading = false;
		}
	}

	async saveSnippet(content: string): Promise<boolean> {
		if (this.snippetInFlight) return false;
		this.snippetInFlight = true;
		this.snippetError = null;
		this.snippetErrorCode = null;
		try {
			await put<CloudInitSnippetResponse>(`${this.#basePath}/snippet`, { content });
			await this.loadSnippet();
			return this.snippetError === null;
		} catch (err) {
			this.snippetErrorCode = err instanceof ApiRequestError ? err.code : null;
			this.snippetError = errorMessage(err, () => m['vms.cloudinit.errorSaveSnippet']());
			return false;
		} finally {
			this.snippetInFlight = false;
		}
	}

	async addSSHKey(key: string, user?: string): Promise<boolean> {
		if (this.sshKeyInFlight) return false;
		this.sshKeyInFlight = true;
		this.sshKeyError = null;
		this.sshKeyErrorCode = null;
		try {
			const body: { key: string; user?: string } = { key };
			if (user && user.trim() !== '') body.user = user.trim();
			await post<CloudInitSSHKeyResponse>(`${this.#basePath}/ssh-keys`, body);
			await this.loadConfig();
			return this.sshKeyError === null;
		} catch (err) {
			this.sshKeyErrorCode = err instanceof ApiRequestError ? err.code : null;
			this.sshKeyError = errorMessage(err, () => m['vms.cloudinit.errorAddSSHKey']());
			return false;
		} finally {
			this.sshKeyInFlight = false;
		}
	}
}

function sanitizeConfig(config: CloudInitConfig): CloudInitConfig {
	return {
		user: config.user,
		sshKeys: [...config.sshKeys],
		ipMode: config.ipMode,
		...(config.ipAddress === undefined ? {} : { ipAddress: config.ipAddress }),
		...(config.gateway === undefined ? {} : { gateway: config.gateway }),
		...(config.dnsServer === undefined ? {} : { dnsServer: config.dnsServer }),
		...(config.searchDomain === undefined ? {} : { searchDomain: config.searchDomain })
	};
}

function errorMessage(err: unknown, fallback: () => string): string {
	return err instanceof ApiRequestError ? err.message : fallback();
}
