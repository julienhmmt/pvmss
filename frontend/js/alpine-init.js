/**
 * Alpine.js initialization and global configuration
 * This file must be loaded BEFORE Alpine.js
 */

document.addEventListener('alpine:init', () => {
  /**
   * Global notification store
   * Usage: Alpine.store('notifications').add({ type: 'success', message: 'Done!' })
   */
  Alpine.store('notifications', {
    items: [],
    counter: 0,

    add(notification) {
      const id = ++this.counter;
      const item = {
        id,
        type: notification.type || 'info',
        message: notification.message || '',
        title: notification.title || '',
        icon: notification.icon || this.getDefaultIcon(notification.type),
        dismissible: notification.dismissible !== false,
        duration: notification.duration || 5000
      };
      this.items.push(item);

      if (notification.autoDismiss !== false && item.duration > 0) {
        setTimeout(() => this.remove(id), item.duration);
      }
      return id;
    },

    remove(id) {
      const index = this.items.findIndex(n => n.id === id);
      if (index > -1) {
        this.items.splice(index, 1);
      }
    },

    clear() {
      this.items = [];
    },

    getDefaultIcon(type) {
      const icons = {
        success: 'fas fa-check-circle',
        danger: 'fas fa-exclamation-circle',
        warning: 'fas fa-exclamation-triangle',
        info: 'fas fa-info-circle'
      };
      return icons[type] || icons.info;
    }
  });

  /**
   * Global loading state store
   * Usage: Alpine.store('loading').set('vmAction', true)
   */
  Alpine.store('loading', {
    states: {},

    set(key, value) {
      this.states[key] = value;
    },

    get(key) {
      return this.states[key] || false;
    },

    toggle(key) {
      this.states[key] = !this.states[key];
    }
  });

  /**
   * Magic helper for CSRF token
   * Usage in Alpine: this.$csrf or $csrf
   */
  Alpine.magic('csrf', () => {
    const input = document.querySelector('input[name="csrf_token"]');
    if (input) return input.value;
    const meta = document.querySelector('meta[name="csrf-token"]');
    return meta ? meta.getAttribute('content') : '';
  });

  /**
   * Magic helper for translations from data attributes
   * Usage: $t('key') where key is from data-t-key attribute
   */
  Alpine.magic('t', (el) => {
    return (key) => {
      const configEl = document.getElementById('alpine-config') || el.closest('[data-translations]');
      if (configEl && configEl.dataset[key]) {
        return configEl.dataset[key];
      }
      return key;
    };
  });
});

/**
 * Dropdown component
 * Usage: x-data="dropdown()"
 */
window.dropdown = () => ({
  open: false,

  toggle() {
    this.open = !this.open;
  },

  close() {
    this.open = false;
  },

  handleClickOutside(event) {
    if (!this.$el.contains(event.target)) {
      this.close();
    }
  }
});

/**
 * Dismissible notification component
 * Usage: x-data="dismissible()"
 */
window.dismissible = (autoDismiss = false, delay = 6000) => ({
  show: true,
  progress: 100,
  interval: null,

  init() {
    if (autoDismiss && delay > 0) {
      const step = 100 / (delay / 50);
      this.interval = setInterval(() => {
        this.progress -= step;
        if (this.progress <= 0) {
          this.dismiss();
        }
      }, 50);
    }
  },

  dismiss() {
    if (this.interval) {
      clearInterval(this.interval);
    }
    this.show = false;
  },

  destroy() {
    if (this.interval) {
      clearInterval(this.interval);
    }
  }
});

/**
 * Tabs component
 * Usage: x-data="tabs('defaultTab')"
 */
window.tabs = (defaultTab = '') => ({
  activeTab: defaultTab,

  isActive(tab) {
    return this.activeTab === tab;
  },

  setActive(tab) {
    this.activeTab = tab;
  }
});

/**
 * Loading button component
 * Usage: x-data="loadingButton()"
 */
window.loadingButton = () => ({
  loading: false,

  start() {
    this.loading = true;
  },

  stop() {
    this.loading = false;
  },

  submit(event) {
    if (this.loading) {
      event.preventDefault();
      return false;
    }
    this.start();
    return true;
  }
});

/**
 * Modal component
 * Usage: x-data="modal()"
 */
window.modal = () => ({
  isOpen: false,

  open() {
    this.isOpen = true;
    document.body.classList.add('is-clipped');
  },

  close() {
    this.isOpen = false;
    document.body.classList.remove('is-clipped');
  },

  toggle() {
    if (this.isOpen) {
      this.close();
    } else {
      this.open();
    }
  }
});
