import { defineComponent, ref, onMounted, onUnmounted, watchEffect, h } from 'vue';

export const NavUserMenu = defineComponent({
  name: 'NavUserMenu',
  props: {
    username:     { type: String, required: true },
    isAdmin:      { type: Boolean, default: false },
    labelAdmin:   { type: String, default: 'Admin' },
    labelProfile: { type: String, default: 'Profile' },
    labelLogout:  { type: String, default: 'Logout' },
    containerEl:  { type: Object, default: null },
  },
  setup(props) {
    const open = ref(false);

    function toggle() { open.value = !open.value; }
    function close() { open.value = false; }

    function onKeydown(e) {
      if (e.key === 'Escape') close();
    }

    function onDocClick(e) {
      if (props.containerEl && !props.containerEl.contains(e.target)) close();
    }

    onMounted(() => {
      window.addEventListener('keydown', onKeydown);
      document.addEventListener('click', onDocClick);
    });

    onUnmounted(() => {
      window.removeEventListener('keydown', onKeydown);
      document.removeEventListener('click', onDocClick);
    });

    // Sync is-active class on the container for CSS-driven chevron rotation
    watchEffect(() => {
      if (props.containerEl) {
        props.containerEl.classList.toggle('is-active', open.value);
      }
    });

    return () => [
      h('button', {
        type: 'button',
        class: 'button is-primary navbar-user-toggle',
        title: 'User menu',
        'aria-expanded': String(open.value),
        'aria-haspopup': 'true',
        onClick: toggle,
      }, [
        h('span', { class: 'icon' }, [h('i', { class: 'fas fa-user-circle' })]),
        h('span', { class: 'navbar-username' }, props.username),
        h('span', { class: 'icon' }, [
          h('i', {
            class: ['fas', 'fa-chevron-down', { 'fa-rotate-180': open.value }],
            style: 'transition: transform 0.2s;',
          }),
        ]),
      ]),
      h('div', {
        class: 'navbar-user-dropdown',
        style: { display: open.value ? undefined : 'none' },
      }, [
        ...(props.isAdmin ? [
          h('a', { class: 'navbar-dropdown-item', href: '/admin/nodes' }, [
            h('span', { class: 'icon' }, [h('i', { class: 'fas fa-cog' })]),
            h('span', null, props.labelAdmin),
          ]),
          h('hr', { class: 'navbar-dropdown-divider' }),
        ] : []),
        h('a', { class: 'navbar-dropdown-item', href: '/profile' }, [
          h('span', { class: 'icon' }, [h('i', { class: 'fas fa-user' })]),
          h('span', null, props.labelProfile),
        ]),
        h('a', { class: 'navbar-dropdown-item', href: '/logout' }, [
          h('span', { class: 'icon' }, [h('i', { class: 'fas fa-sign-out-alt' })]),
          h('span', null, props.labelLogout),
        ]),
      ]),
    ];
  },
});
