import { beforeEach, describe, expect, it } from 'vitest';
import { mount } from 'svelte';
import DashboardAttention from './DashboardAttention.svelte';
import type { DashboardAlert } from './dashboard-alerts';

const infoAlert: DashboardAlert = {
	kind: 'node_offline_disabled',
	severity: 'info',
	cluster: 'east',
	clusterKey: 'east',
	subject: 'n2'
};

const warningAlert: DashboardAlert = {
	kind: 'node_cpu',
	severity: 'warning',
	cluster: 'east',
	clusterKey: 'east',
	subject: 'n1',
	percent: 95
};

beforeEach(() => {
	document.body.innerHTML = '';
});

describe('DashboardAttention', () => {
	it('shows the all-clear line above the info rows when nothing is actionable', () => {
		mount(DashboardAttention, { target: document.body, props: { alerts: [infoAlert] } });

		expect(document.querySelector('[data-testid="dashboard-all-clear"]')).not.toBeNull();

		const rows = document.querySelectorAll('[data-testid="dashboard-alert"]');
		expect(rows).toHaveLength(1);
		expect(rows.item(0)?.getAttribute('data-severity')).toBe('info');
		expect(rows.item(0)?.getAttribute('data-kind')).toBe('node_offline_disabled');
	});

	it('hides the all-clear line when an actionable alert is present', () => {
		mount(DashboardAttention, { target: document.body, props: { alerts: [warningAlert, infoAlert] } });

		expect(document.querySelector('[data-testid="dashboard-all-clear"]')).toBeNull();
		expect(document.querySelectorAll('[data-testid="dashboard-alert"]')).toHaveLength(2);
	});

	it('shows only the all-clear line when there are no alerts', () => {
		mount(DashboardAttention, { target: document.body, props: { alerts: [] } });

		expect(document.querySelector('[data-testid="dashboard-all-clear"]')).not.toBeNull();
		expect(document.querySelector('[data-testid="dashboard-alert"]')).toBeNull();
	});
});
