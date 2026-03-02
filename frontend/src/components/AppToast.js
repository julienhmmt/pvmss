import { defineComponent, h } from 'vue';
import { useNotificationsStore } from '../stores/notifications.js';

const TYPE_CLASS = {
  success: 'is-success',
  danger:  'is-danger',
  warning: 'is-warning',
  info:    'is-info',
};
const TYPE_ICON = {
  success: 'fa-check-circle',
  danger:  'fa-exclamation-circle',
  warning: 'fa-exclamation-triangle',
  info:    'fa-info-circle',
};

export const AppToast = defineComponent({
  name: 'AppToast',
  setup() {
    const store = useNotificationsStore();
    return () => h('div', {
      class: 'app-toast-container',
      style: 'position:fixed;top:1rem;right:1rem;z-index:9999;width:320px;',
    },
      store.items.map(n =>
        h('div', {
          key: n.id,
          class: ['notification', TYPE_CLASS[n.type] || 'is-info'],
          style: 'margin-bottom:0.5rem;',
        }, [
          n.dismissible && h('button', { class: 'delete', onClick: () => store.remove(n.id) }),
          h('span', { class: 'icon' }, [h('i', { class: ['fas', TYPE_ICON[n.type] || 'fa-info-circle'] })]),
          n.title && h('strong', null, ` ${n.title} `),
          h('span', null, n.message),
        ])
      )
    );
  },
});
