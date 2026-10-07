import { describe, it, expect } from 'vitest';
import { alertHref, alertMessage, usageTone, type DashboardAlert } from './dashboard-alerts';

const alert = (kind: DashboardAlert['kind'], extra: Partial<DashboardAlert> = {}): DashboardAlert => ({
	kind,
	severity: 'warning',
	cluster: 'east',
	...extra
});

describe('dashboard alerts', () => {
	it('sends each alert to the page where it is fixed', () => {
		expect(alertHref(alert('cluster_unreachable'))).toBe('/admin/clusters');
		// clusterKey wins over the display label when the server sends both.
		expect(alertHref(alert('node_offline', { subject: 'n1', cluster: 'East Campus', clusterKey: 'east' }))).toBe('/admin/nodes/east/n1');
		expect(alertHref(alert('node_offline_disabled', { subject: 'n1' }))).toBe('/admin/nodes/east/n1');
		expect(alertHref(alert('node_cpu', { subject: 'n1' }))).toBe('/admin/nodes/east/n1');
		expect(alertHref(alert('node_memory', { subject: 'n1' }))).toBe('/admin/nodes/east/n1');
	});

	it('deep-links storage and pool alerts to the filtered admin list', () => {
		expect(alertHref(alert('storage_full', { subject: 'ceph' }))).toBe('/admin/storages?cluster=east&search=ceph');
		expect(alertHref(alert('storage_full', { subject: 'ceph', cluster: 'East Campus', clusterKey: 'east' }))).toBe('/admin/storages?cluster=east&search=ceph');
		expect(alertHref(alert('pool_at_quota', { subject: 'p1' }))).toBe('/admin/policy?cluster=east');
	});

	it('names the subject and the figure in the message', () => {
		expect(alertMessage(alert('storage_full', { subject: 'ceph', percent: 96 }))).toContain('ceph');
		expect(alertMessage(alert('storage_full', { subject: 'ceph', percent: 96 }))).toContain('96');
		expect(alertMessage(alert('cluster_unreachable'))).toContain('east');
	});

	it('describes an offline node that is disabled in PVMSS', () => {
		const message = alertMessage(alert('node_offline_disabled', { severity: 'info', subject: 'n2' }));
		expect(message).toContain('n2');
		expect(message).not.toBe(alertMessage(alert('node_offline', { subject: 'n2' })));
	});

	it('tones usage like the alert thresholds', () => {
		expect(usageTone(50)).toBe('success');
		expect(usageTone(85)).toBe('warning');
		expect(usageTone(95)).toBe('destructive');
	});
});
