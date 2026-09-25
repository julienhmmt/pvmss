import { m } from '$lib/paraglide/messages.js';

/**
 * publishingOffHint localizes why cloud-init publishing is off for a cluster
 * (the server's publishingStatus code), so the admin knows which prerequisite
 * to fix.
 */
export function publishingOffHint(status: string | undefined): string {
	switch (status) {
		case 'no_ssh_key':
			return m['admin.clusters.publishingOff.noSshKey']();
		case 'no_ssh_user':
			return m['admin.clusters.publishingOff.noSshUser']();
		case 'no_host_keys':
			return m['admin.clusters.publishingOff.noHostKeys']();
		case 'no_snippet_storage':
			return m['admin.clusters.publishingOff.noSnippetStorage']();
		default:
			return m['admin.clusters.cloudinitOff']();
	}
}
