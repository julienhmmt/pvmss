import { defineComponent, h } from 'vue';
import { VmActionButtons } from './VmActionButtons.js';

const STATUS_CLASS = {
  running: 'vm-card--running',
  stopped: 'vm-card--stopped',
};

export const VmCard = defineComponent({
  name: 'VmCard',
  props: {
    vm: { type: Object, required: true },
  },
  emits: ['action'],
  setup(props, { emit }) {
    return () => {
      const { vmid, name, node, status, mem_mb, max_mem_mb, disk_mb, cpus } = props.vm;
      return h('div', { class: ['vm-card', STATUS_CLASS[status] || ''] }, [
        h('div', { class: 'vm-card__header' }, [
          h('span', { class: 'vm-card__name' }, name || `VM ${vmid}`),
          h('span', { class: `vm-card__status vm-card__status--${status}` }, status),
        ]),
        h('div', { class: 'vm-card__meta' }, [
          h('span', null, `Node: ${node}`),
          h('span', null, `CPU: ${cpus}`),
          h('span', null, `RAM: ${mem_mb}/${max_mem_mb} MB`),
          h('span', null, `Disk: ${disk_mb} MB`),
        ]),
        h(VmActionButtons, {
          vmid,
          node,
          status,
          onAction: (payload) => emit('action', payload),
        }),
      ]);
    };
  },
});
