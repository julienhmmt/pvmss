import { createRouter, createWebHistory } from 'vue-router';
import { VmListPage }    from './pages/VmListPage.js';
import { SearchPage }    from './pages/SearchPage.js';
import { ProfilePage }   from './pages/ProfilePage.js';
import { VmDetailsPage } from './pages/VmDetailsPage.js';
import { LoginPage }     from './pages/LoginPage.js';

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/',                 component: VmListPage },
    { path: '/search',           component: SearchPage },
    { path: '/profile',          component: ProfilePage },
    { path: '/vm/details/:vmid', component: VmDetailsPage },
    { path: '/login',            component: LoginPage },
    // admin routes remain server-rendered — no vue route needed
  ],
});
