import { m } from '#lib/paraglide/messages.js';
import type { QuotaRequest } from '#lib/features/quota-requests/quota-requests.js';

export interface AppNotification {
	id: string;
	kind: 'success' | 'error' | 'info';
	message: string;
	/** ISO timestamp; unread when later than the user's last look. */
	at: string;
	href?: string;
}

const MAX_ITEMS = 20;
const SEEN_KEY_PREFIX = 'pvmss-notifications-seen-v1:';

/** A decided quota request as a notification; null while pending. */
export function quotaNotification(
	request: QuotaRequest | null
): AppNotification | null {
	if (request === null || request.status === 'pending') return null;
	const approved = request.status === 'approved';
	return {
		id: `quota-${request.id}-${request.status}`,
		kind: approved ? 'success' : 'info',
		message: approved
			? m['vms.create.quotaRequest.approved']({ quota: request.requestedQuota })
			: m['vms.create.quotaRequest.denied'](),
		at: request.decidedAt || request.createdAt
	};
}

/**
* The header bell's feed for a pool user: quota decisions (from the server,
* so they survive reloads) and task outcomes seen this session.
* shortcut: task outcomes are session-only; a server-side feed would be needed to see ones that ended while signed out.
*/
export class NotificationsState {
	items = $state.raw<readonly AppNotification[]>([]);
	#seenAt = $state('');
	readonly #user: string;

	constructor(user: string) {
		this.#user = user;
		try {
			this.#seenAt = localStorage.getItem(SEEN_KEY_PREFIX + user) ?? '';
		} catch {
			this.#seenAt = '';
		}
	}

	get unread(): number {
		return this.items.filter((item) => item.at > this.#seenAt).length;
	}

	add(item: AppNotification | null): void {
		if (item === null || this.items.some((existing) => existing.id === item.id))
			return;
		this.items = [item, ...this.items]
			.sort((a, b) => b.at.localeCompare(a.at))
			.slice(0, MAX_ITEMS);
	}

	// eslint-disable-next-line svelte/prefer-svelte-reactivity -- a one-off timestamp, never mutated
	markSeen(now: string = new Date().toISOString()): void {
		this.#seenAt = now;
		try {
			localStorage.setItem(SEEN_KEY_PREFIX + this.#user, now);
		} catch {
			// Blocked storage: the badge clears for this page view only.
		}
	}
}
