import { get, post, put, del, ApiRequestError } from '$lib/shared/api/client';
import { fetchClusterOptions, type ClusterOption } from '$lib/shared/clusters';
import { setContext, getContext } from 'svelte';
import { m } from '$lib/paraglide/messages.js';

/** One node's outcome for a published document. */
export interface NodePublication {
	node: string;
	ok: boolean;
	error?: string;
}

/** The latest publication of a template: the immutable file PVMSS wrote
 *  over SSH to every node, and whether each node lists it. */
export interface Publication {
	filename: string;
	publishedAt: string;
	nodes: NodePublication[];
}

export interface AdminCloudInitTemplate {
	id: string;
	label: string;
	content: string;
	enabled: boolean;
	publication: Publication | null;
	/** Set when saving tried to publish and could not start at all. */
	publishError?: string;
}

/** True when every node lists the published file. */
export function fullyPublished(publication: Publication | null): boolean {
	return publication !== null && publication.nodes.length > 0 && publication.nodes.every((n) => n.ok);
}

/**
 * AdminCloudInitTemplatesStore manages the CRUD state for cloud-init
 * templates. API responses are $state.raw - they are API data, not form edits.
 */
export class AdminCloudInitTemplatesStore {
	templates = $state.raw<AdminCloudInitTemplate[]>([]);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	saving = $state.raw(false);
	saveError = $state.raw<string | null>(null);
	clusterOptions = $state.raw<ClusterOption[]>([]);
	cluster = $state('default');
	publishing = $state.raw(false);
	/** Warning after a save or a "publish all" (not configured, node failures). */
	publishWarning = $state.raw<string | null>(null);

	/** Resolves the real cluster name (matches admin-catalog's pattern) so
	 *  requests never send the literal "default" against a deployment whose
	 *  cluster is named something else. */
	async loadClusters(): Promise<void> {
		try {
			this.clusterOptions = await fetchClusterOptions();
			const first = this.clusterOptions[0];
			if (first && (this.clusterOptions.length === 1 || !this.clusterOptions.some((option) => option.name === this.cluster))) {
				this.cluster = first.name;
			}
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.cloudinit.loadError']();
		}
	}

	setCluster(value: string): void {
		this.cluster = value;
		void this.load();
	}

	async load(): Promise<void> {
		await this.loadClusters();
		this.loading = true;
		this.error = null;
		try {
			this.templates = await get<AdminCloudInitTemplate[]>(
				`/api/v1/admin/cloudinit-templates?cluster=${encodeURIComponent(this.cluster)}`
			);
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.cloudinit.loadError']();
		} finally {
			this.loading = false;
		}
	}

	async create(label: string, content: string): Promise<void> {
		this.saving = true;
		this.saveError = null;
		try {
			const created = await post<AdminCloudInitTemplate>('/api/v1/admin/cloudinit-templates', {
				cluster: this.cluster,
				label,
				content
			});
			this.templates = [...this.templates, created];
			this.publishWarning = publicationWarning(created);
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.cloudinit.createError']();
			throw err;
		} finally {
			this.saving = false;
		}
	}

	async update(id: string, label: string, content: string): Promise<void> {
		this.saving = true;
		this.saveError = null;
		try {
			const updated = await put<AdminCloudInitTemplate>(
				`/api/v1/admin/cloudinit-templates/${id}`,
				{ cluster: this.cluster, label, content }
			);
			this.templates = this.templates.map((t) => (t.id === id ? updated : t));
			this.publishWarning = publicationWarning(updated);
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.cloudinit.updateError']();
			throw err;
		} finally {
			this.saving = false;
		}
	}

	async remove(id: string): Promise<void> {
		this.saveError = null;
		try {
			await del<{ status: string }>(`/api/v1/admin/cloudinit-templates/${id}?cluster=${encodeURIComponent(this.cluster)}`);
			this.templates = this.templates.filter((t) => t.id !== id);
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.cloudinit.deleteError']();
			throw err;
		}
	}

	async toggle(id: string, enabled: boolean): Promise<void> {
		this.saveError = null;
		try {
			await post<{ id: string; enabled: boolean }>(
				`/api/v1/admin/cloudinit-templates/${id}/toggle`,
				{ cluster: this.cluster, enabled }
			);
			this.templates = this.templates.map((t) => (t.id === id ? { ...t, enabled } : t));
			// Enabling publishes the template: reload to show where it landed.
			if (enabled) await this.load();
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.cloudinit.toggleError']();
			throw err;
		}
	}

	/** Republishes the baseline and every enabled template to every node
	 *  (after adding or reinstalling a node, or upgrading PVMSS). */
	async publishAll(): Promise<void> {
		this.publishing = true;
		this.publishWarning = null;
		try {
			const result = await post<{ error?: string }>(
				`/api/v1/admin/cloudinit-templates/publish?cluster=${encodeURIComponent(this.cluster)}`,
				{}
			);
			if (result.error) this.publishWarning = result.error;
			await this.load();
			if (!this.publishWarning && this.templates.some((t) => t.enabled && !fullyPublished(t.publication))) {
				this.publishWarning = m['admin.cloudinit.publishPartial']();
			}
		} catch (err) {
			this.publishWarning = err instanceof ApiRequestError ? err.message : m['admin.cloudinit.publishError']();
		} finally {
			this.publishing = false;
		}
	}
}

const CLOUDINIT_TEMPLATES_CONTEXT_KEY = Symbol('admin-cloudinit-templates');

export function setAdminCloudInitTemplatesContext(): AdminCloudInitTemplatesStore {
	const store = new AdminCloudInitTemplatesStore();
	setContext(CLOUDINIT_TEMPLATES_CONTEXT_KEY, store);
	return store;
}

export function getAdminCloudInitTemplatesContext(): AdminCloudInitTemplatesStore {
	return getContext<AdminCloudInitTemplatesStore>(CLOUDINIT_TEMPLATES_CONTEXT_KEY);
}

function publicationWarning(template: AdminCloudInitTemplate): string | null {
	if (template.publishError) return m['admin.cloudinit.publishNotConfigured']({ error: template.publishError });
	if (template.enabled && !fullyPublished(template.publication)) return m['admin.cloudinit.publishPartial']();
	return null;
}
