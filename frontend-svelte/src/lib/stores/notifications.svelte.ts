import type { Notification } from '$lib/types/navbar';

interface NotificationState {
	notifications: Notification[];
	unreadCount: number;
}

interface NotificationStore {
	notifications: Notification[];
	unreadCount: number;
	add(notification: Omit<Notification, 'id' | 'timestamp' | 'read'>): void;
	remove(id: string): void;
	markAsRead(id: string): void;
	markAllAsRead(): void;
	clear(): void;
}

function createNotificationStore(): NotificationStore {
	let state = $state<NotificationState>({
		notifications: [],
		unreadCount: 0
	});

	return {
		get notifications() {
			return state.notifications;
		},
		get unreadCount() {
			return state.unreadCount;
		},

		add(notification: Omit<Notification, 'id' | 'timestamp' | 'read'>) {
			const newNotification: Notification = {
				...notification,
				id: crypto.randomUUID(),
				timestamp: new Date(),
				read: false
			};
			state.notifications = [newNotification, ...state.notifications];
			state.unreadCount = state.notifications.filter((n) => !n.read).length;
		},

		remove(id: string) {
			state.notifications = state.notifications.filter((n) => n.id !== id);
			state.unreadCount = state.notifications.filter((n) => !n.read).length;
		},

		markAsRead(id: string) {
			state.notifications = state.notifications.map((n) =>
				n.id === id ? { ...n, read: true } : n
			);
			state.unreadCount = state.notifications.filter((n) => !n.read).length;
		},

		markAllAsRead() {
			state.notifications = state.notifications.map((n) => ({ ...n, read: true }));
			state.unreadCount = 0;
		},

		clear() {
			state.notifications = [];
			state.unreadCount = 0;
		}
	};
}

export const notifications = createNotificationStore();
