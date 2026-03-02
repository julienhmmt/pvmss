import { defineComponent, h } from 'vue';

export const AppModal = defineComponent({
  name: 'AppModal',
  props: {
    title: { type: String, default: '' },
    modelValue: { type: Boolean, default: false },
  },
  emits: ['update:modelValue'],
  setup(props, { slots, emit }) {
    function close() {
      emit('update:modelValue', false);
      document.body.style.overflow = '';
    }
    return () => props.modelValue
      ? h('div', { class: 'modal is-active' }, [
          h('div', { class: 'modal-background', onClick: close }),
          h('div', { class: 'modal-card' }, [
            h('header', { class: 'modal-card-head' }, [
              h('p', { class: 'modal-card-title' }, props.title),
              h('button', { class: 'delete', onClick: close }),
            ]),
            h('section', { class: 'modal-card-body' }, slots.default?.()),
            slots.footer && h('footer', { class: 'modal-card-foot' }, slots.footer()),
          ]),
        ])
      : null;
  },
});
