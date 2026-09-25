import { get, post, put, ApiRequestError } from '$lib/shared/api/client';
import { m } from '$lib/paraglide/messages.js';
import type {
	CloudInitConfig,
	CloudInitConfigUpdate,
	CloudInitDocument,
	CloudInitSSHKeyResponse,
	CloudInitTemplateOption
} from './cloudinit.types';

interface CloudInitUpdateResponse {
	status: string;
	rebooted: boolean;
}

interface CloudInitDocumentResponse {
	status: string;
}

interface TemplateCatalog {
	cloudInitTemplates: CloudInitTemplateOption[];
	cloudInitWriteEnabled: boolean;
	cloudInitBaselineId: string;
}

export class CloudInitStore {
	readonly cluster: string;
	readonly vmid: number;

	config = $state.raw<CloudInitConfig | null>(null);
	document = $state.raw<CloudInitDocument | null>(null);
	/** Published admin templates of the VM's cluster. */
	templates = $state.raw<CloudInitTemplateOption[]>([]);
	/** False when the cluster does not publish cloud-init documents. */
	publishingEnabled = $state.raw(false);
	/** Document id of the standalone baseline, from the cluster catalog. */
	baselineTemplateId = $state.raw('');
	configLoading = $state.raw(false);
	documentLoading = $state.raw(false);
	configInFlight = $state.raw(false);
	documentInFlight = $state.raw(false);
	sshKeyInFlight = $state.raw(false);
	configError = $state.raw<string | null>(null);
	documentError = $state.raw<string | null>(null);
	documentErrorCode = $state.raw<string | null>(null);
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

	/** Loads the VM's document and the cluster's published templates. */
	async loadDocument(): Promise<void> {
		this.documentLoading = true;
		this.documentError = null;
		this.documentErrorCode = null;
		try {
			const [document, catalog] = await Promise.all([
				get<CloudInitDocument>(`${this.#basePath}/document`),
				get<TemplateCatalog>(`/api/v1/vm-create/catalog?cluster=${encodeURIComponent(this.cluster)}`)
			]);
			this.document = document;
			this.templates = catalog.cloudInitTemplates ?? [];
			this.publishingEnabled = catalog.cloudInitWriteEnabled;
			this.baselineTemplateId = catalog.cloudInitBaselineId ?? '';
		} catch (err) {
			this.documentError = errorMessage(err, () => m['vms.cloudinit.errorLoadDocument']());
		} finally {
			this.documentLoading = false;
		}
	}

	/** Switches the VM to a published template ('' detaches). Nothing is
	 *  written by the user: the file was published by an administrator. */
	async saveDocument(templateId: string): Promise<boolean> {
		if (this.documentInFlight) return false;
		this.documentInFlight = true;
		this.documentError = null;
		this.documentErrorCode = null;
		try {
			await put<CloudInitDocumentResponse>(`${this.#basePath}/document`, { templateId });
			this.document = await get<CloudInitDocument>(`${this.#basePath}/document`);
			return true;
		} catch (err) {
			this.documentErrorCode = err instanceof ApiRequestError ? err.code : null;
			this.documentError = documentErrorMessage(err);
			return false;
		} finally {
			this.documentInFlight = false;
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

// sanitizeConfig whitelists the fields the UI may hold. The GET never returns
// write-only fields (password), but the store must not surface one if the API
// ever does - a test asserts `password` never reaches store state. It also
// copies sshKeys so the reactive config never aliases the response array.
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

function documentErrorMessage(err: unknown): string {
	if (err instanceof ApiRequestError) {
		if (err.code === 'cloudinit_write_unavailable') return m['vms.cloudinit.errorWriteUnavailable']();
		if (err.code === 'cloudinit_not_published') return m['vms.cloudinit.errorNotPublished']();
		return err.message;
	}
	return m['vms.cloudinit.errorSaveDocument']();
}
