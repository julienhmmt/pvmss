import { api } from '$lib/api/client';
import type { Node } from '$lib/types/admin';

export function getNodes(): Promise<Node[]> {
	return api.get('/api/v1/admin/nodes');
}
