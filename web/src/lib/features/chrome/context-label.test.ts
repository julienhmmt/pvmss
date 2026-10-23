import { beforeAll, describe, expect, it } from 'vitest';
import { setLocale } from '#lib/paraglide/runtime.js';
import { contextLabel } from './context-label';

beforeAll(() => setLocale('en', { reload: false }));

describe('contextLabel', () => {
	it.each([
		['/', 'Workspace', ''],
		['/vms', 'Workspace', 'My machines'],
		['/vms/', 'Workspace', 'My machines'],
		['/vms////', 'Workspace', 'My machines'],
		['/vms/create', 'Workspace', 'Create a machine'],
		['/vms/default/102', 'Workspace', 'Machine'],
		['/vms/default/102/console', 'Workspace', 'Console'],
		['/activity', 'Workspace', 'Activity'],
		['/docs', 'Workspace', 'Help & guides'],
		['/docs/user-guide', 'Workspace', 'Help & guides'],
		['/profile', 'Workspace', 'Your account'],
		['/search', 'Workspace', 'Search'],
		['/about', 'Workspace', 'About'],
		['/nodes', 'Workspace', 'Nodes'],
		['/admin', 'Administration', 'Dashboard'],
		['/admin/nodes', 'Administration', 'Nodes'],
		['/admin/clusters', 'Administration', 'Clusters'],
		['/admin/pools', 'Administration', 'PVMSS users'],
		['/admin/storages', 'Administration', 'Storages'],
		['/admin/isos', 'Administration', 'ISOs'],
		['/admin/templates', 'Administration', 'VM templates'],
		['/admin/images', 'Administration', 'Cloud images'],
		['/admin/bridges', 'Administration', 'Bridges'],
		['/admin/cloudinit-templates', 'Administration', 'Cloud-init'],
		['/admin/baseline', 'Administration', 'Cloud-init baseline'],
		['/admin/docs', 'Administration', 'Documentation'],
		['/admin/profiles', 'Administration', 'VM Profiles'],
		['/admin/tags', 'Administration', 'Tags'],
		['/admin/policy', 'Administration', 'Limits'],
		['/admin/policy/nodes', 'Administration', 'Node capacity'],
		['/admin/appinfo', 'Administration', 'App Info'],
		['/admin/settings', 'Administration', 'Settings'],
		['/admin/unknown', 'Administration', ''],
		['/somewhere', 'Workspace', '']
	])('%s -> %s / %s', (path, section, screen) => {
		expect(contextLabel(path)).toEqual({ section, screen });
	});
});
