import { api } from "./client";

export interface AuthUser {
  username: string;
  is_admin: boolean;
}

// Use raw fetch so a 401 (unauthenticated) doesn't trigger the client's
// redirect-to-login logic. Failing exchange is normal for logged-out users.
export async function exchange(): Promise<AuthUser> {
  const res = await fetch("/api/v1/auth/exchange", {
    method: "POST",
    credentials: "same-origin",
  });
  if (!res.ok) throw new Error("not authenticated");
  return res.json();
}

export async function me(): Promise<AuthUser> {
  return api.get<AuthUser>("/api/v1/auth/me");
}

export async function logout(): Promise<void> {
  return api.post("/api/v1/auth/logout");
}

export async function login(
  username: string,
  password: string,
): Promise<AuthUser> {
  return api.post<AuthUser>("/api/v1/auth/login", {
    username,
    password,
    admin: false,
  });
}

export async function adminLogin(password: string): Promise<AuthUser> {
  return api.post<AuthUser>("/api/v1/auth/login", {
    username: "admin",
    password,
    admin: true,
  });
}

export async function proxmoxAdminLogin(
  username: string,
  password: string,
): Promise<AuthUser> {
  return api.post<AuthUser>("/api/v1/auth/proxmox-admin-login", {
    username,
    password,
  });
}
