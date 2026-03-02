import { defineComponent, h } from 'vue';

export const AppTabs = defineComponent({
  name: 'AppTabs',
  props: {
    tabs: { type: Array, required: true }, // [{ key, label }]
    modelValue: { type: String, required: true },
  },
  emits: ['update:modelValue'],
  setup(props, { slots, emit }) {
    return () => h('div', null, [
      h('div', { class: 'tabs' },
        h('ul', null, props.tabs.map(tab =>
          h('li', { class: props.modelValue === tab.key ? 'is-active' : '' },
            h('a', { onClick: () => emit('update:modelValue', tab.key) }, tab.label)
          )
        ))
      ),
      slots[props.modelValue]?.(),
    ]);
  },
});
