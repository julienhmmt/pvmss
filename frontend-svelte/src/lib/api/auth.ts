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

export async function login(username: string, password: string): Promise<AuthUser> {
	return api.post<AuthUser>('/api/v1/auth/login', { username, password, admin: false });
}

export async function adminLogin(password: string): Promise<AuthUser> {
	return api.post<AuthUser>('/api/v1/auth/login', { username: 'admin', password, admin: true });
}

export async function proxmoxAdminLogin(username: string, password: string): Promise<AuthUser> {
	return api.post<AuthUser>('/api/v1/auth/proxmox-admin-login', { username, password });
}
