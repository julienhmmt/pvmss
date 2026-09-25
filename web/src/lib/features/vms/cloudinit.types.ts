/** Cloud-init types shared by the VM cloud-init tab and its store. */

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

/** The cloud-init document a VM uses: an admin template (templateId) or a
 *  legacy per-VM document written before documents became admin-published.
 *  The standalone baseline's id comes from the cluster catalog
 *  (CloudInitStore.baselineTemplateId), never hardcoded here. */
export interface CloudInitDocument {
	templateId: string | null;
	filename: string | null;
	legacy: boolean;
	updatedAt: string | null;
	updatedBy: string | null;
}

/** One admin template the user may switch the VM to. */
export interface CloudInitTemplateOption {
	id: string;
	label: string;
}

export interface CloudInitSSHKeyResponse {
	status: string;
}
