import { describe, it, expect } from 'vitest';
import { alertHref, alertMessage, hasActionableAlerts, usageTone, type DashboardAlert } from './dashboard-alerts';

const alert = (kind: DashboardAlert['kind'], extra: Partial<DashboardAlert> = {}): DashboardAlert => ({
	kind,
	severity: 'warning',
	cluster: 'east',
	...extra
});

describe('dashboard alerts', () => {
	it('counts only critical and warning alerts as actionable', () => {
		expect(hasActionableAlerts([])).toBe(false);
		expect(hasActionableAlerts([alert('node_cpu', { severity: 'info' })])).toBe(false);
		expect(hasActionableAlerts([alert('node_cpu', { severity: 'info' }), alert('node_cpu')])).toBe(true);
		expect(hasActionableAlerts([alert('cluster_unreachable', { severity: 'critical' })])).toBe(true);
	});

	it('sends each alert to the page where it is fixed', () => {
		expect(alertHref(alert('cluster_unreachable'))).toBe('/admin/clusters/east?tab=connection');
		// clusterKey wins over the display label when the server sends both.
		expect(alertHref(alert('node_offline', { subject: 'n1', cluster: 'East Campus', clusterKey: 'east' }))).toBe('/admin/nodes/east/n1?from=dashboard');
		expect(alertHref(alert('node_cpu', { subject: 'n1' }))).toBe('/admin/nodes/east/n1?from=dashboard');
		expect(alertHref(alert('node_memory', { subject: 'n1' }))).toBe('/admin/nodes/east/n1?from=dashboard');
	});

	it('deep-links storage and pool alerts to the filtered admin list', () => {
		expect(alertHref(alert('storage_full', { subject: 'ceph' }))).toBe('/admin/storages?cluster=east&search=ceph');
		expect(alertHref(alert('storage_full', { subject: 'ceph', cluster: 'East Campus', clusterKey: 'east' }))).toBe('/admin/storages?cluster=east&search=ceph');
		expect(alertHref(alert('pool_at_quota', { subject: 'p1' }))).toBe('/admin/policy?cluster=east');
	});

	it('links a failed snapshot schedule to its VM', () => {
		const failed = alert('snapshot_schedule_failed', { subject: '101', cluster: 'East Campus', clusterKey: 'east' });
		expect(alertHref(failed)).toBe('/vms/east/101?tab=snapshots');
		expect(alertMessage(failed)).toContain('101');
	});

	it('names the subject and the figure in the message', () => {
		expect(alertMessage(alert('storage_full', { subject: 'ceph', percent: 96 }))).toContain('ceph');
		expect(alertMessage(alert('storage_full', { subject: 'ceph', percent: 96 }))).toContain('96');
		expect(alertMessage(alert('cluster_unreachable'))).toContain('east');
	});

	it('sends readiness and unadopted alerts to where they are fixed', () => {
		expect(alertHref(alert('node_incomplete', { subject: 'n1', detail: 'bridge' }))).toBe('/admin/bridges?cluster=east');
		expect(alertHref(alert('node_incomplete', { subject: 'n1', detail: 'iso_or_template' }))).toBe('/admin/isos?cluster=east');
		expect(alertHref(alert('node_unadopted_vms', { subject: 'n1', used: 2 }))).toBe('/admin/recovery?cluster=east');
		expect(alertMessage(alert('node_unadopted_vms', { subject: 'n1', used: 2 }))).toContain('2');
		expect(alertMessage(alert('node_incomplete', { subject: 'n1' }))).toContain('n1');
	});

	it('tones usage like the alert thresholds', () => {
		expect(usageTone(50)).toBe('success');
		expect(usageTone(85)).toBe('warning');
		expect(usageTone(95)).toBe('destructive');
	});
});
