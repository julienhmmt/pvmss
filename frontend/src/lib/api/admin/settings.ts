import { api } from "$lib/api/client";
import type { AppSettings } from "$lib/types/settings";
import { transformKeysToCamelCase, transformKeysToSnakeCase } from "$lib/utils/transform";

/**
 * Fetches the application settings from the admin API.
 * Transforms snake_case response from backend to camelCase for frontend.
 * @returns Promise<AppSettings> - The application settings object
 */
export async function getSettings(): Promise<AppSettings> {
  const response = await api.get<Record<string, unknown>>("/api/v1/admin/settings");
  return transformKeysToCamelCase<AppSettings>(response);
}

export const getAdminSettings = getSettings;

/**
 * Saves the application settings to the admin API.
 * Transforms camelCase frontend data to snake_case for backend.
 * @param settings - The application settings to save
 * @returns Promise<void>
 */
export async function saveSettings(settings: AppSettings): Promise<void> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>(settings);
  return api.put<void>("/api/v1/admin/settings", payload);
}
