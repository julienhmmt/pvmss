import { createApp, defineComponent, h } from 'vue';
import { createPinia } from 'pinia';
import { RouterView } from 'vue-router';
import { router } from './router.js';
import { AppToast } from './components/AppToast.js';
import { useAuthStore } from './stores/auth.js';
import { NavUserMenu } from './components/NavUserMenu.js';

// root app: global toast + router-view
const RootApp = defineComponent({
  name: 'RootApp',
  setup: () => () => h('div', null, [
    h(AppToast),
    h(RouterView),
  ]),
});

// Mount NavUserMenu on each navbar user menu element (desktop + mobile)
document.querySelectorAll('[data-vue-component="nav-user-menu"]').forEach(el => {
  const { username, isAdmin, labelAdmin, labelProfile, labelLogout } = el.dataset;
  createApp(NavUserMenu, {
    username,
    isAdmin: isAdmin === 'true',
    labelAdmin,
    labelProfile,
    labelLogout,
    containerEl: el,
  }).mount(el);
});

const mountEl = document.getElementById('vue-app');
if (mountEl) {
  const app = createApp(RootApp);
  const pinia = createPinia();
  app.use(pinia);
  app.use(router);

  const authStore = useAuthStore();
  authStore.init(mountEl).then(() => {
    app.mount(mountEl);
  });
}
