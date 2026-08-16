import { get } from '$lib/shared/api/client';

/** DocSummary mirrors the docSummaryDTO from GET /api/v1/docs. */
export interface DocSummary {
	id: string;
	lang: string;
	title: string;
	category: string;
	audience: 'user' | 'admin';
}

/** DocRendered mirrors the docRenderedDTO from GET /api/v1/docs/{id}. */
export interface DocRendered {
	id: string;
	lang: string;
	title: string;
	html: string;
}

/** fetchDocPage is a one-shot helper for the viewer route (no store needed). */
export async function fetchDocPage(id: string, lang: string): Promise<DocRendered> {
	const params = new URLSearchParams({ lang });
	return get<DocRendered>(`/api/v1/docs/${id}?${params.toString()}`);
}
