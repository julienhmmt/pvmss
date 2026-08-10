// Ambient Window augmentation for T10's console E2E test hook.
//
// __pvmssForceConsoleBoundaryError is read once by VmConsole.svelte to force a
// render-time throw, exercising the <svelte:boundary> failed snippet in
// +page.svelte (SC-004). Set only by Playwright via page.addInitScript() —
// never by application code, never reachable from a URL. See VmConsole.svelte
// for why this is a window global instead of a query parameter.
export {};

declare global {
	interface Window {
		__pvmssForceConsoleBoundaryError?: boolean;
	}
}
