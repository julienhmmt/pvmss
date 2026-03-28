import { exchange, me } from '$lib/api/auth';

interface AuthState {
	username: string;
	isAdmin: boolean;
	initialized: boolean;
}

function createAuthStore() {
	let state = $state<AuthState>({
		username: '',
		isAdmin: false,
		initialized: false
	});

	return {
		get username() {
			return state.username;
		},
		get isAdmin() {
			return state.isAdmin;
		},
		get initialized() {
			return state.initialized;
		},

		async exchange() {
			try {
				const user = await exchange();
				state = { username: user.username, isAdmin: user.is_admin, initialized: true };
			} catch {
				state = { username: '', isAdmin: false, initialized: true };
				window.location.href = '/login';
			}
		},

		async refresh() {
			try {
				const user = await me();
				state = { ...state, username: user.username, isAdmin: user.is_admin };
			} catch {
				// Token expired, exchange will handle redirect
			}
		},

		clear() {
			state = { username: '', isAdmin: false, initialized: true };
		}
	};
}

export const auth = createAuthStore();
