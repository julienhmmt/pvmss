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
		expect(alertHref(alert('node_offline', { subject: 'n1' }))).toBe('/nodes');
		expect(alertHref(alert('node_cpu', { subject: 'n1' }))).toBe('/nodes');
		expect(alertHref(alert('node_memory', { subject: 'n1' }))).toBe('/nodes');
		expect(alertHref(alert('storage_full', { subject: 'ceph' }))).toBe('/admin/storages');
		expect(alertHref(alert('pool_at_quota', { subject: 'p1' }))).toBe('/admin/policy');
	});

	it('names the subject and the figure in the message', () => {
		expect(alertMessage(alert('storage_full', { subject: 'ceph', percent: 96 }))).toContain('ceph');
		expect(alertMessage(alert('storage_full', { subject: 'ceph', percent: 96 }))).toContain('96');
		expect(alertMessage(alert('cluster_unreachable'))).toContain('east');
	});

	it('tones usage like the alert thresholds', () => {
		expect(usageTone(50)).toBe('success');
		expect(usageTone(85)).toBe('warning');
		expect(usageTone(95)).toBe('destructive');
	});
});
