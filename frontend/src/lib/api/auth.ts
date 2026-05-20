import { api } from "./client";
import type { User } from "$lib/types/auth";
import { transformKeysToCamelCase, transformKeysToSnakeCase } from "$lib/utils/transform";

export type AuthUser = User;

// Use raw fetch so a 401 (unauthenticated) doesn't trigger the client's
// redirect-to-login logic. Failing exchange is normal for logged-out users.
export async function exchange(): Promise<AuthUser> {
  // Note: access_token is HttpOnly so it is never visible via document.cookie.
  // We always attempt the exchange and let the server determine auth state.
  const res = await fetch("/api/v1/auth/exchange", {
    method: "POST",
    credentials: "same-origin",
  });
  if (!res.ok) throw new Error("not authenticated");
  const data = await res.json();
  return transformKeysToCamelCase<AuthUser>(data);
}

export async function me(): Promise<AuthUser> {
  const data = await api.get<AuthUser>("/api/v1/auth/me");
  return transformKeysToCamelCase<AuthUser>(data);
}

export async function logout(): Promise<void> {
  return api.post("/api/v1/auth/logout");
}

export async function login(
  username: string,
  password: string,
): Promise<AuthUser> {
  const data = await api.post<AuthUser>(
    "/api/v1/auth/login",
    transformKeysToSnakeCase({ username, password, admin: false }),
  );
  return transformKeysToCamelCase<AuthUser>(data);
}

export async function adminLogin(password: string): Promise<AuthUser> {
  const data = await api.post<AuthUser>(
    "/api/v1/auth/login",
    transformKeysToSnakeCase({ username: "admin", password, admin: true }),
  );
  return transformKeysToCamelCase<AuthUser>(data);
}

export async function proxmoxAdminLogin(
  username: string,
  password: string,
): Promise<AuthUser> {
  const data = await api.post<AuthUser>(
    "/api/v1/auth/proxmox-admin-login",
    transformKeysToSnakeCase({ username, password }),
  );
  return transformKeysToCamelCase<AuthUser>(data);
}

export async function changePassword(
  current: string,
  newPassword: string,
): Promise<void> {
  await api.put<void>(
    "/api/v1/auth/me/password",
    transformKeysToSnakeCase({ currentPassword: current, newPassword }),
  );
}
