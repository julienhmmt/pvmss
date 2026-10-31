import { describe, expect, it } from 'vitest';
import { AuditLogStore } from './auditLog.svelte';

describe('AuditLogStore.exportUrl', () => {
	it('carries the filter but not the pagination', () => {
		const store = new AuditLogStore();
		store.filter = { action: 'start', vmid: 101, page: 3, pageSize: 50 };
		expect(store.exportUrl).toBe('/api/v1/admin/audit/export?vmid=101&action=start');
	});
});
