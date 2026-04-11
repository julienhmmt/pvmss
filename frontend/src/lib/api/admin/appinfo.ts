import { api } from "$lib/api/client";
import type { AppInfo } from "$lib/types/admin";

export function getAppInfo(): Promise<AppInfo> {
  return api.get("/api/v1/admin/appinfo");
}
