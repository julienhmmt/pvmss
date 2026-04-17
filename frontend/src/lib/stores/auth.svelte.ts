import { exchange, me } from "$lib/api/auth";

interface AuthState {
  username: string;
  isAdmin: boolean;
  initialized: boolean;
}

interface AuthStore {
  username: string;
  isAdmin: boolean;
  initialized: boolean;
  exchange(): Promise<void>;
  refresh(): Promise<void>;
  setUser(username: string, isAdmin: boolean): void;
  clear(): void;
}

function createAuthStore(): AuthStore {
  let state = $state<AuthState>({
    username: "",
    isAdmin: false,
    initialized: false,
  });

  return {
    get username() {
      return state.username;
    },
    get isAdmin() {
      return state.isAdmin;
    },
    get initialized() {
      return state.initialized;
    },

    async exchange() {
      try {
        const user = await exchange();
        state = {
          username: user.username,
          isAdmin: user.isAdmin,
          initialized: true,
        };
      } catch (err) {
        // Expected for first-time visitors or logged-out users - not an error
        if (err instanceof Error && err.message === "not authenticated") {
          console.debug("[AuthStore] User not authenticated");
        } else {
          console.error("[AuthStore] Token exchange failed:", err);
        }
        state = { username: "", isAdmin: false, initialized: true };
      }
    },

    async refresh() {
      try {
        const user = await me();
        state = { ...state, username: user.username, isAdmin: user.isAdmin };
      } catch (err) {
        console.error("[AuthStore] Token refresh failed:", err);
        // Token expired, exchange will handle redirect
      }
    },

    setUser(username: string, isAdmin: boolean) {
      state = { username, isAdmin, initialized: true };
    },

    clear() {
      state = { username: "", isAdmin: false, initialized: true };
    },
  };
}

export const auth = createAuthStore();
