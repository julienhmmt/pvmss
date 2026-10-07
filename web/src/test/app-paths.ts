// vitest mock for $app/paths - resolve() substitutes [param] segments like the
// real implementation since tests don't use a base path.
export function resolve(pathname: string, params?: Record<string, string>): string {
	if (!params) return pathname;
	return pathname.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`);
}

export const base = '';
