import { defineComponent, h, ref, onMounted, onUnmounted } from 'vue';

export const AppDropdown = defineComponent({
  name: 'AppDropdown',
  setup(_, { slots }) {
    const open = ref(false);
    function close(e) {
      if (!e.target.closest('.dropdown')) open.value = false;
    }
    onMounted(() => document.addEventListener('click', close));
    onUnmounted(() => document.removeEventListener('click', close));
    return () => h('div', { class: ['dropdown', open.value ? 'is-active' : ''] }, [
      h('div', { class: 'dropdown-trigger' },
        h('div', { onClick: () => { open.value = !open.value; } }, slots.trigger?.())
      ),
      h('div', { class: 'dropdown-menu' },
        h('div', { class: 'dropdown-content' }, slots.default?.())
      ),
    ]);
  },
});
