import { defineComponent, h, onMounted } from 'vue';
import { useVMStore } from './stores/vms.js';
import { VmCard } from './components/VmCard.js';

export const App = defineComponent({
  name: 'App',
  setup() {
    const vmStore = useVMStore();

    onMounted(() => vmStore.fetchVMs());

    function handleAction({ vmid, action, node }) {
      vmStore.doAction(vmid, action, node);
    }

    return () => {
      if (vmStore.loading) {
        return h('div', { class: 'vm-list vm-list--loading' }, 'Loading VMs…');
      }
      if (vmStore.error) {
        return h('div', { class: 'vm-list vm-list--error' }, vmStore.error);
      }
      if (vmStore.vms.length === 0) {
        return h('div', { class: 'vm-list vm-list--empty' }, 'No VMs found.');
      }
      return h('div', { class: 'vm-list' },
        vmStore.vms.map(vm =>
          h(VmCard, {
            key: vm.vmid,
            vm,
            onAction: handleAction,
          })
        )
      );
    };
  },
});
