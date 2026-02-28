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
    try {
      const { data } = await me();
      username.value = data.username;
      isAdmin.value = data.is_admin;
    } catch (_) {
      // JWT not yet available — data-attributes are still valid for display
    }
    ready.value = true;
  }

  return { username, isAdmin, ready, init };
});
