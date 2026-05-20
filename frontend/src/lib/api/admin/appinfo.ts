import { api } from "$lib/api/client";
import type { AppInfo } from "$lib/types/admin";
import { transformKeysToCamelCase } from "$lib/utils/transform";

export async function getAppInfo(): Promise<AppInfo> {
  const response = await api.get<AppInfo>("/api/v1/admin/appinfo");
  return transformKeysToCamelCase<AppInfo>(response);
}
