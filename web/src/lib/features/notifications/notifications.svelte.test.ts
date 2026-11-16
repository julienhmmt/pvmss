import { beforeEach, describe, expect, it } from 'vitest';
import {
	NotificationsState,
	quotaNotification,
	type AppNotification
} from './notifications.svelte';
import type { QuotaRequest } from '#lib/features/quota-requests/quota-requests.js';

const note = (id: string, at: string): AppNotification => ({
	id,
	kind: 'info',
	message: id,
	at
});

const request = (status: QuotaRequest['status']): QuotaRequest => ({
	id: 7,
	requester: 'alice',
	cluster: 'c',
	pool: 'p',
	currentQuota: 3,
	requestedQuota: 6,
	reason: '',
	status,
	decidedBy: 'admin',
	decidedAt: '2026-11-16T10:00:00Z',
	createdAt: '2026-11-15T10:00:00Z'
});

describe('NotificationsState', () => {
	beforeEach(() => localStorage.clear());

	it('counts items newer than the last look, per user', () => {
		const state = new NotificationsState('alice');
		state.add(note('a', '2026-11-16T09:00:00Z'));
		state.add(note('b', '2026-11-16T11:00:00Z'));
		expect(state.unread).toBe(2);
		state.markSeen('2026-11-16T10:00:00Z');
		expect(state.unread).toBe(1);
		expect(new NotificationsState('alice').unread).toBe(0);
		expect(localStorage.getItem('pvmss-notifications-seen-v1:bob')).toBeNull();
	});

	it('keeps newest first and ignores duplicates', () => {
		const state = new NotificationsState('alice');
		state.add(note('a', '2026-11-16T09:00:00Z'));
		state.add(note('b', '2026-11-16T11:00:00Z'));
		state.add(note('a', '2026-11-16T12:00:00Z'));
		state.add(null);
		expect(state.items.map((i) => i.id)).toEqual(['b', 'a']);
	});
});

describe('quotaNotification', () => {
	it('is null while pending and tags the decision otherwise', () => {
		expect(quotaNotification(null)).toBeNull();
		expect(quotaNotification(request('pending'))).toBeNull();
		expect(quotaNotification(request('approved'))).toMatchObject({
			id: 'quota-7-approved',
			kind: 'success',
			at: '2026-11-16T10:00:00Z'
		});
		expect(quotaNotification(request('denied'))?.kind).toBe('info');
	});
});
