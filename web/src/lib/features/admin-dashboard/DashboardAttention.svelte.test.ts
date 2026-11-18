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
	it('lists every alert, info rows included', () => {
		mount(DashboardAttention, { target: document.body, props: { alerts: [warningAlert, infoAlert] } });

		const rows = document.querySelectorAll('[data-testid="dashboard-alert"]');
		expect(rows).toHaveLength(2);
		expect(rows.item(1)?.getAttribute('data-severity')).toBe('info');
		expect(rows.item(1)?.getAttribute('data-kind')).toBe('node_offline_disabled');
	});
});
