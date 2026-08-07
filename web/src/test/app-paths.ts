// vitest mock for $app/paths — resolve() just returns the pathname as-is
// since tests don't use a base path.
export function resolve(pathname: string): string {
	return pathname;
}

export const base = '';
