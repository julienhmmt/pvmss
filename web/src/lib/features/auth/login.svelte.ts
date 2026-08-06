import { post } from '$lib/shared/api/client';

export type LoginProvider = 'pve' | 'local';

export interface Principal {
	username: string;
	pool: string;
	isAdmin: boolean;
}

export class LoginForm {
	username = $state.raw('');
	password = $state.raw('');
	provider = $state.raw<LoginProvider>('pve');
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);

	async submit(): Promise<Principal | null> {
		this.loading = true;
		this.error = null;
		try {
			if (this.provider === 'local') {
				return await post<Principal>('/api/v1/auth/admin-login', { password: this.password });
			}
			return await post<Principal>('/api/v1/auth/login', { username: this.username, password: this.password });
		} catch (error: unknown) {
			this.error = error instanceof Error ? error.message : 'unable to sign in';
			return null;
		} finally {
			this.loading = false;
		}
	}
}
