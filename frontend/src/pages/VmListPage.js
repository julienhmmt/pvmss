import { defineComponent, h, onMounted } from 'vue';
import { useVMStore } from '../stores/vms.js';
import { VmCard } from '../components/VmCard.js';

export const VmListPage = defineComponent({
  name: 'VmListPage',
  setup() {
    const vmStore = useVMStore();
    onMounted(() => vmStore.fetchVMs());

    return () => {
      if (vmStore.loading) return h('div', { class: 'has-text-centered p-6' }, 'Loading VMs\u2026');
      if (vmStore.error) return h('div', { class: 'notification is-danger' }, vmStore.error);
      if (!vmStore.vms.length) return h('div', { class: 'notification is-warning' }, 'No VMs found.');
      return h('div', { class: 'vm-list columns is-multiline' },
        vmStore.vms.map(vm =>
          h('div', { key: vm.vmid, class: 'column is-12-mobile is-6-tablet is-4-desktop' },
            h(VmCard, {
              vm,
              onAction: ({ vmid, action, node }) => vmStore.doAction(vmid, action, node),
            })
          )
        )
      );
    };
  },
});
