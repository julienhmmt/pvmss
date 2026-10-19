import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';
import { m } from '#lib/paraglide/messages.js';
import type { AuditEntry } from './auditLog.svelte';
import AuditLogHarness from './AuditLogHarness.test.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

async function settle(): Promise<void> {
	for (let i = 0; i < 5; i += 1) {
		await Promise.resolve();
		flushSync();
	}
}

const entry: AuditEntry = {
	id: 1,
	actor: 'alice',
	cluster: 'default',
	vmid: 101,
	action: 'vm.start',
	timestamp: '2025-01-01T10:00:00Z',
	targetType: 'vm',
	targetId: '101',
	detail: '{"summary":"started vm","ok":true}',
	ipAddress: '10.0.0.1',
	severity: 'info'
};

function auditPage(items: AuditEntry[]): { items: AuditEntry[]; total: number; page: number; pageSize: number } {
	return { items, total: items.length, page: 1, pageSize: 20 };
}

describe('AuditLogPanel', () => {
	afterEach(() => {
		document.body.innerHTML = '';
		vi.restoreAllMocks();
		vi.unstubAllGlobals();
	});

	it('renders entries without the empty state and without a page title', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, auditPage([entry]))));
		const app = mount(AuditLogHarness, { target: document.body });
		await settle();

		expect(document.body.textContent).toContain('alice');
		expect(document.body.textContent).not.toContain(m['admin.audit.empty']());
		// The panel must not clobber the settings page title.
		expect(document.title).toBe('');
		await unmount(app);
	});

	it('renders the empty state only when there are no entries', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, auditPage([]))));
		const app = mount(AuditLogHarness, { target: document.body });
		await settle();

		expect(document.body.textContent).toContain(m['admin.audit.empty']());
		await unmount(app);
	});

	it('expands a full-width detail row when an entry is clicked', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, auditPage([entry]))));
		const app = mount(AuditLogHarness, { target: document.body });
		await settle();

		const toggle = document.body.querySelector<HTMLButtonElement>('tbody button');
		expect(toggle).not.toBeNull();
		toggle!.click();
		await settle();

		const detailCell = document.body.querySelector<HTMLTableCellElement>('td[colspan="8"]');
		expect(detailCell).not.toBeNull();
		expect(detailCell!.querySelector('pre')?.textContent).toContain('"ok": true');
		await unmount(app);
	});
});
