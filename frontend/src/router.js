import { createRouter, createWebHistory } from 'vue-router';
import { VmListPage }    from './pages/VmListPage.js';
import { SearchPage }    from './pages/SearchPage.js';
import { ProfilePage }   from './pages/ProfilePage.js';
import { VmDetailsPage } from './pages/VmDetailsPage.js';

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/',                 component: VmListPage },
    { path: '/search',           component: SearchPage },
    { path: '/profile',          component: ProfilePage },
    { path: '/vm/details/:vmid', component: VmDetailsPage },
    // /login and all admin routes are server-rendered — not Vue routes
  ],
});
