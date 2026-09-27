// vitest mock for $app/navigation - goto is a no-op in unit tests.
export async function goto(_url: string | URL): Promise<void> {
	// no-op
}

/** No-op in unit tests - navigation lifecycle hooks never fire. */
export function afterNavigate(_callback: unknown): void {
	// no-op
}
