/**
 * Navigation link interface
 */
export interface NavLink {
	href: string;
	icon: any;
	label: string;
	authRequired: boolean;
}

/**
 * Notification interface
 */
export interface Notification {
	id: string;
	type: 'info' | 'warning' | 'error' | 'success';
	title: string;
	message: string;
	timestamp: Date;
	read: boolean;
}

/**
 * Status indicator interface
 */
export interface StatusIndicator {
	id: string;
	name: string;
	status: 'connected' | 'disconnected' | 'warning' | 'unknown';
	tooltip?: string;
}

/**
 * Keyboard shortcut interface
 */
export interface KeyboardShortcut {
	key: string;
	description: string;
	action: () => void;
	macKey?: string;
	ctrlKey?: boolean;
	shiftKey?: boolean;
	altKey?: boolean;
}
