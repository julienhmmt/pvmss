import { defineStore } from 'pinia';
import { ref } from 'vue';
import { me } from '../api/auth.js';

export const useAuthStore = defineStore('auth', () => {
  const username = ref('');
  const isAdmin = ref(false);
  const ready = ref(false);

  async function init(mountEl) {
    // Bootstrap from data attributes injected by the Go template
    if (mountEl) {
      username.value = mountEl.dataset.username || '';
      isAdmin.value = mountEl.dataset.isAdmin === 'true';
    }
    // Exchange session cookie for JWT tokens before any API calls
    try { await fetch('/api/v1/auth/exchange', { method: 'POST', credentials: 'include' }); } catch (_) {}
    try {
      const { data } = await me();
      username.value = data.username;
      isAdmin.value = data.is_admin;
    } catch (_) {
      // Not authenticated or JWT unavailable
    }
    ready.value = true;
  }

  return { username, isAdmin, ready, init };
});
