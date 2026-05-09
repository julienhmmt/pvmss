import { getVMCreateSettings } from "$lib/api/vm-create";
import type { VMCreateSettings } from "$lib/types/vm-create";

interface SettingsState {
  settings: VMCreateSettings | null;
  loading: boolean;
  error: Error | null;
  lastFetch: number | null;
}

interface SettingsStore {
  settings: VMCreateSettings | null;
  loading: boolean;
  error: Error | null;
  fetchSettings(force?: boolean): Promise<VMCreateSettings>;
  clear(): void;
}

function createSettingsStore(): SettingsStore {
  let state = $state<SettingsState>({ settings: null, loading: false, error: null, lastFetch: null });
  const CACHE_TTL = 60000; // 60 seconds
  return {
    get settings() {
      return state.settings;
    },
    get loading() {
      return state.loading;
    },
    get error() {
      return state.error;
    },
    async fetchSettings(force = false) {
      const now = Date.now();
      const isStale = state.lastFetch === null || (now - state.lastFetch) > CACHE_TTL;
      if (!force && !isStale && state.settings !== null) return state.settings;
      state = { ...state, loading: true, error: null };
      try {
        const settings = await getVMCreateSettings();
        state = { settings, loading: false, error: null, lastFetch: now };
        return settings;
      } catch (error) {
        const err = error instanceof Error ? error : new Error("Failed to fetch settings");
        state = { ...state, loading: false, error: err };
        throw err;
      }
    },
    clear() {
      state = { settings: null, loading: false, error: null, lastFetch: null };
    },
  };
}

export const settingsStore = createSettingsStore();
