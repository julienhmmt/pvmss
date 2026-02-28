import { defineComponent, h } from 'vue';

export const AppButton = defineComponent({
  name: 'AppButton',
  props: {
    variant: { type: String, default: 'default' },
    loading: { type: Boolean, default: false },
    disabled: { type: Boolean, default: false },
  },
  emits: ['click'],
  setup(props, { slots, emit }) {
    return () => h('button', {
      class: ['app-btn', `app-btn--${props.variant}`],
      disabled: props.disabled || props.loading,
      onClick: (e) => emit('click', e),
    }, props.loading ? '…' : slots.default?.());
  },
});
