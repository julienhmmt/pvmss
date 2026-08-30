// Node 22+ exposes an experimental global `localStorage` that is undefined
// unless `--localstorage-file` is passed on the CLI, and recent happy-dom
// versions no longer populate a working `localStorage` in the vitest
// environment either. The unit tests for locale/theme/draft rely on a real
// Storage; install a minimal in-memory implementation when the global is
// missing so those tests behave as they did in the browser.
function createMemoryStorage(): Storage {
	let store = new Map<string, string>();
	const storage: Storage = {
		get length(): number {
			return store.size;
		},
		clear(): void {
			store = new Map();
		},
		getItem(key: string): string | null {
			return store.has(key) ? store.get(key)! : null;
		},
		key(index: number): string | null {
			return Array.from(store.keys())[index] ?? null;
		},
		removeItem(key: string): void {
			store.delete(key);
		},
		setItem(key: string, value: string): void {
			store.set(key, String(value));
		}
	};
	return storage;
}

if (typeof globalThis.localStorage === 'undefined') {
	try {
		Object.defineProperty(globalThis, 'localStorage', {
			value: createMemoryStorage(),
			writable: true,
			configurable: true
		});
	} catch {
		// Some environments expose a non-configurable accessor — fall back to
		// plain assignment where the property is absent.
		globalThis.localStorage = createMemoryStorage();
	}
}
