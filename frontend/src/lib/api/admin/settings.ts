import { api } from "$lib/api/client";
import type { AppSettings } from "$lib/types/settings";

/**
 * Fetches the application settings from the admin API.
 * @returns Promise<AppSettings> - The application settings object
 */
export function getSettings(): Promise<AppSettings> {
  return api.get<AppSettings>("/api/v1/admin/settings");
}
