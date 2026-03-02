import { defineComponent, h, ref } from 'vue';
import { useRouter } from 'vue-router';
import { useNotificationsStore } from '../stores/notifications.js';
import { AppTabs } from '../components/AppTabs.js';
import { login } from '../api/auth.js';

export const LoginPage = defineComponent({
  name: 'LoginPage',
  setup() {
    const router   = useRouter();
    const notif    = useNotificationsStore();
    const activeTab    = ref('local');
    const username     = ref('admin');
    const password     = ref('');
    const pveUsername  = ref('');
    const pvePassword  = ref('');
    const loading      = ref(false);

    async function handleLocalLogin(e) {
      e.preventDefault();
      loading.value = true;
      try {
        await login(username.value, password.value, true);
        router.push('/');
      } catch (err) {
        notif.add({ type: 'danger', message: err.response?.data?.message || 'Login failed' });
      } finally {
        loading.value = false;
      }
    }

    async function handlePveLogin(e) {
      e.preventDefault();
      loading.value = true;
      const user = pveUsername.value.includes('@') ? pveUsername.value : pveUsername.value + '@pve';
      try {
        await login(user, pvePassword.value, false);
        router.push('/profile');
      } catch (err) {
        notif.add({ type: 'danger', message: err.response?.data?.message || 'Proxmox login failed' });
      } finally {
        loading.value = false;
      }
    }

    const localForm = () => h('form', { onSubmit: handleLocalLogin }, [
      h('div', { class: 'field' }, [
        h('label', { class: 'label' }, 'Username'),
        h('div', { class: 'control' },
          h('input', {
            class: 'input',
            type: 'text',
            value: username.value,
            onInput: e => { username.value = e.target.value; },
          })
        ),
      ]),
      h('div', { class: 'field' }, [
        h('label', { class: 'label' }, 'Password'),
        h('div', { class: 'control' },
          h('input', {
            class: 'input',
            type: 'password',
            value: password.value,
            onInput: e => { password.value = e.target.value; },
          })
        ),
      ]),
      h('div', { class: 'field' },
        h('button', {
          class: ['button', 'is-primary', loading.value && 'is-loading'],
          type: 'submit',
          disabled: loading.value,
        }, 'Login')
      ),
    ]);

    const pveForm = () => h('form', { onSubmit: handlePveLogin }, [
      h('div', { class: 'field' }, [
        h('label', { class: 'label' }, 'Proxmox Username'),
        h('div', { class: 'control' },
          h('input', {
            class: 'input',
            type: 'text',
            value: pveUsername.value,
            onInput: e => { pveUsername.value = e.target.value; },
            onBlur: e => {
              const v = e.target.value;
              if (v && !v.includes('@')) pveUsername.value = v + '@pve';
            },
          })
        ),
      ]),
      h('div', { class: 'field' }, [
        h('label', { class: 'label' }, 'Password'),
        h('div', { class: 'control' },
          h('input', {
            class: 'input',
            type: 'password',
            value: pvePassword.value,
            onInput: e => { pvePassword.value = e.target.value; },
          })
        ),
      ]),
      h('div', { class: 'field' },
        h('button', {
          class: ['button', 'is-primary', loading.value && 'is-loading'],
          type: 'submit',
          disabled: loading.value,
        }, 'Login with Proxmox')
      ),
    ]);

    return () => h('section', { class: 'section' },
      h('div', { class: 'container' },
        h('div', { class: 'columns is-centered' },
          h('div', { class: 'column is-half' },
            h('div', { class: 'card' }, [
              h('header', { class: 'card-header' },
                h('p', { class: 'card-header-title' }, 'Sign in to PVMSS')
              ),
              h('div', { class: 'card-content' },
                h(AppTabs, {
                  tabs: [{ key: 'local', label: 'Admin' }, { key: 'pve', label: 'Proxmox User' }],
                  modelValue: activeTab.value,
                  'onUpdate:modelValue': v => { activeTab.value = v; },
                }, { local: localForm, pve: pveForm })
              ),
            ])
          )
        )
      )
    );
  },
});
