import { get, ApiRequestError } from '$lib/shared/api/client';

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

/**
 * DocsBrowserStore fetches the public, audience-filtered documentation list
 * and the rendered-HTML single-page view. Admin-audience pages are hidden by
 * the server for non-admin callers; the store simply renders what it receives.
 */
export class DocsBrowserStore {
	pages = $state.raw<DocSummary[]>([]);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.pages = await get<DocSummary[]>('/api/v1/docs');
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : 'failed to load documentation';
		} finally {
			this.loading = false;
		}
	}

	async fetchPage(id: string, lang: string): Promise<DocRendered> {
		const params = new URLSearchParams({ lang });
		return get<DocRendered>(`/api/v1/docs/${id}?${params.toString()}`);
	}
}

/** fetchDocPage is a one-shot helper for the viewer route (no store needed). */
export async function fetchDocPage(id: string, lang: string): Promise<DocRendered> {
	const params = new URLSearchParams({ lang });
	return get<DocRendered>(`/api/v1/docs/${id}?${params.toString()}`);
}
