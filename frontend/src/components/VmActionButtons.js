import { defineComponent, h, ref } from 'vue';
import { AppButton } from './AppButton.js';

const ACTIONS = [
  { action: 'start',    label: 'Start',    variant: 'success', forStatus: ['stopped'] },
  { action: 'stop',     label: 'Stop',     variant: 'danger',  forStatus: ['running'] },
  { action: 'shutdown', label: 'Shutdown', variant: 'warning', forStatus: ['running'] },
  { action: 'reboot',   label: 'Reboot',   variant: 'info',    forStatus: ['running'] },
];

export const VmActionButtons = defineComponent({
  name: 'VmActionButtons',
  props: {
    vmid: { type: Number, required: true },
    node: { type: String, required: true },
    status: { type: String, required: true },
  },
  emits: ['action'],
  setup(props, { emit }) {
    const pending = ref('');

    async function handleAction(action) {
      pending.value = action;
      try {
        await emit('action', { vmid: props.vmid, action, node: props.node });
      } finally {
        pending.value = '';
      }
    }

    return () => h('div', { class: 'vm-action-buttons' },
      ACTIONS
        .filter(a => a.forStatus.includes(props.status))
        .map(({ action, label, variant }) =>
          h(AppButton, {
            key: action,
            variant,
            loading: pending.value === action,
            onClick: () => handleAction(action),
          }, () => label)
        )
    );
  },
});
