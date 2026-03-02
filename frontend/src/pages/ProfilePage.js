import { defineComponent, h } from 'vue';
import { useProfileStore } from '../stores/profile.js';
import { useAuthStore } from '../stores/auth.js';
import { useAutoRefresh } from '../composables/useAutoRefresh.js';
import { VmCard } from '../components/VmCard.js';

export const ProfilePage = defineComponent({
  name: 'ProfilePage',
  setup() {
    const store = useProfileStore();
    const auth = useAuthStore();
    const { loading, error } = useAutoRefresh(() => store.fetchMyVMs(), 30000);

    return () => h('section', { class: 'section' },
      h('div', { class: 'container' }, [
        h('h1', { class: 'title' }, `My VMs \u2014 ${auth.username}`),
        loading.value && h('p', null, 'Loading\u2026'),
        error.value && h('div', { class: 'notification is-danger' }, error.value),
        !loading.value && store.vms.length === 0
          ? h('div', { class: 'notification is-warning' }, 'No VMs found.')
          : h('div', { class: 'columns is-multiline' },
              store.vms.map(vm =>
                h('div', { key: vm.vmid, class: 'column is-12-mobile is-6-tablet is-4-desktop' },
                  h(VmCard, {
                    vm,
                    onAction: ({ vmid, action, node }) => store.doAction(vmid, action, node),
                  })
                )
              )
            ),
      ])
    );
  },
});
