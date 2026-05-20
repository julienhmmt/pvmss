import { api } from "$lib/api/client";
import type { Node } from "$lib/types/admin";
import { transformKeysToCamelCase } from "$lib/utils/transform";

export async function getNodes(): Promise<Node[]> {
  const response = await api.get<Node[]>("/api/v1/admin/nodes");
  return transformKeysToCamelCase<Node[]>(response);
}

export async function toggleNode(name: string): Promise<void> {
  await api.post("/api/v1/admin/nodes/toggle", { name });
}
