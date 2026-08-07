// vitest mock for $app/navigation — goto is a no-op in unit tests.
export async function goto(_url: string | URL): Promise<void> {
	// no-op
}
