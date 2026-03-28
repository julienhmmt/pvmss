import { api } from './client';

export interface AuthUser {
	username: string;
	is_admin: boolean;
}

export async function exchange(): Promise<AuthUser> {
	return api.post<AuthUser>('/api/v1/auth/exchange');
}

export async function me(): Promise<AuthUser> {
	return api.get<AuthUser>('/api/v1/auth/me');
}

export async function logout(): Promise<void> {
	return api.post('/api/v1/auth/logout');
}
