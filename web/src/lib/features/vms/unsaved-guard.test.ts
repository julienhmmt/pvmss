import { describe, expect, it } from 'vitest';
import { guardDecision } from './unsaved-guard';

function nav(to: string | null, willUnload = false): Parameters<typeof guardDecision>[0] {
	return {
		type: willUnload ? 'leave' : 'link',
		willUnload,
		to: to === null ? null : ({ url: new URL(to, 'http://localhost') } as never)
	};
}

describe('guardDecision', () => {
	it('allows any navigation when nothing is dirty', () => {
		expect(guardDecision(nav('/vms'), false, '/vms/default/101')).toBe('allow');
	});

	it('asks in-app before leaving the machine with a dirty draft', () => {
		expect(guardDecision(nav('/vms'), true, '/vms/default/101')).toBe('confirm');
	});

	it('lets tab changes on the same machine through', () => {
		expect(guardDecision(nav('/vms/default/101?tab=hardware'), true, '/vms/default/101')).toBe('allow');
	});

	it('defers to the browser prompt when the page unloads', () => {
		expect(guardDecision(nav(null, true), true, '/vms/default/101')).toBe('native');
	});
});
