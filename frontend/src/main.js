import { createApp } from 'vue';
import { createPinia } from 'pinia';
import { App } from './App.js';
import { useAuthStore } from './stores/auth.js';

const mountEl = document.getElementById('vue-app');
if (mountEl) {
  const app = createApp(App);
  const pinia = createPinia();
  app.use(pinia);

  // Bootstrap auth state from data attributes + /api/v1/auth/me
  const authStore = useAuthStore();
  authStore.init(mountEl).then(() => {
    app.mount(mountEl);
  });
}
