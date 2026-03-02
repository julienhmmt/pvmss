import { defineComponent, h, ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { useVmDetailsStore } from '../stores/vmDetails.js';
import { useAutoRefresh } from '../composables/useAutoRefresh.js';
import { VmActionButtons } from '../components/VmActionButtons.js';
import { AppModal } from '../components/AppModal.js';

function pct(v) { return `${(v || 0).toFixed(1)}%`; }

export const VmDetailsPage = defineComponent({
  name: 'VmDetailsPage',
  setup() {
    const route = useRoute();
    const store = useVmDetailsStore();
    const vmid  = route.params.vmid;

    const showDescModal      = ref(false);
    const showTagsModal      = ref(false);
    const showResourcesModal = ref(false);
    const showCelebration    = ref(window.location.search.includes('created=1'));

    onMounted(() => {
      store.fetchVM(vmid);
      if (showCelebration.value) setTimeout(() => { showCelebration.value = false; }, 10000);
    });

    // auto-refresh metrics every 30s
    useAutoRefresh(() => store.fetchMetrics(vmid), 30000);

    return () => {
      if (store.loading) return h('div', { class: 'has-text-centered p-6' }, 'Loading VM\u2026');
      if (store.error)   return h('div', { class: 'notification is-danger section' }, store.error);
      if (!store.vm)     return null;

      const vm = store.vm;
      return h('section', { class: 'section' },
        h('div', { class: 'container' }, [
          // celebration banner
          showCelebration.value && h('div', { class: 'notification is-success' }, [
            h('button', { class: 'delete', onClick: () => { showCelebration.value = false; } }),
            '\uD83C\uDF89 VM created successfully!',
          ]),
          // header: vm name + action buttons
          h('div', { class: 'level mb-4' }, [
            h('div', { class: 'level-left' }, h('h1', { class: 'title' }, vm.name || `VM ${vmid}`)),
            h('div', { class: 'level-right' },
              h(VmActionButtons, {
                vmid: vm.vmid,
                node: vm.node,
                status: vm.status,
                onAction: ({ vmid: id, action, node }) => store.doAction(id, action, node),
              })
            ),
          ]),
          // metrics card
          store.metrics && h('div', { class: 'card mb-4' },
            h('div', { class: 'card-content' }, h('div', { class: 'columns' }, [
              h('div', { class: 'column' }, [
                h('p', { class: 'heading' }, 'CPU'),
                h('p', { class: 'title is-4' }, pct(store.metrics.cpu_pct)),
              ]),
              h('div', { class: 'column' }, [
                h('p', { class: 'heading' }, 'Memory'),
                h('p', { class: 'title is-4' }, `${store.metrics.mem_mb} / ${store.metrics.max_mem_mb} MB`),
              ]),
              h('div', { class: 'column' }, [
                h('p', { class: 'heading' }, 'Uptime'),
                h('p', { class: 'title is-4' }, `${Math.floor((store.metrics.uptime || 0) / 3600)}h`),
              ]),
            ]))
          ),
          // edit buttons
          h('div', { class: 'buttons mb-4' }, [
            h('button', { class: 'button', onClick: () => { showDescModal.value = true; } }, 'Edit Description'),
            h('button', { class: 'button', onClick: () => { showTagsModal.value = true; } }, 'Edit Tags'),
            h('button', { class: 'button', onClick: () => { showResourcesModal.value = true; } }, 'Edit Resources'),
          ]),
          // modals
          h(AppModal, {
            title: 'Edit Description',
            modelValue: showDescModal.value,
            'onUpdate:modelValue': v => { showDescModal.value = v; },
          }, { default: () => h('p', null, 'Description editor — coming soon') }),
          h(AppModal, {
            title: 'Edit Tags',
            modelValue: showTagsModal.value,
            'onUpdate:modelValue': v => { showTagsModal.value = v; },
          }, { default: () => h('p', null, 'Tags editor — coming soon') }),
          h(AppModal, {
            title: 'Edit Resources',
            modelValue: showResourcesModal.value,
            'onUpdate:modelValue': v => { showResourcesModal.value = v; },
          }, { default: () => h('p', null, 'Resources editor — coming soon') }),
        ])
      );
    };
  },
});
