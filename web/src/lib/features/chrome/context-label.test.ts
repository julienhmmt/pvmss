import { beforeAll, describe, expect, it } from 'vitest';
import { setLocale } from '$lib/paraglide/runtime.js';
import { contextLabel } from './context-label';

beforeAll(() => setLocale('en', { reload: false }));

describe('contextLabel', () => {
	it.each([
		['/vms', 'Workspace', 'My machines'],
		['/vms/', 'Workspace', 'My machines'],
		['/vms/create', 'Workspace', 'Create a machine'],
		['/vms/default/102', 'Workspace', 'Machine'],
		['/vms/default/102/console', 'Workspace', 'Console'],
		['/activity', 'Workspace', 'Activity'],
		['/docs', 'Workspace', 'Help & guides'],
		['/docs/user-guide', 'Workspace', 'Help & guides'],
		['/profile', 'Workspace', 'Your account'],
		['/admin', 'Administration', 'Dashboard'],
		['/admin/policy/nodes', 'Administration', 'Node capacity'],
		['/somewhere', 'Workspace', '']
	])('%s -> %s / %s', (path, section, screen) => {
		expect(contextLabel(path)).toEqual({ section, screen });
	});
});
