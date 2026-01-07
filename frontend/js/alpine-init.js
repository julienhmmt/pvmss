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

/**
 * Auto-refresh component with Visibility API support
 * Usage: x-data="autoRefresh('/api/endpoint', 30000)"
 */
window.autoRefresh = (url, intervalMs = 30000) => ({
  data: null,
  loading: false,
  error: null,
  interval: null,
  enabled: true,
  paused: false,

  init() {
    this.fetch();
    this.start();
    document.addEventListener('visibilitychange', () => this.handleVisibility());
  },

  destroy() {
    this.stop();
  },

  async fetch() {
    if (this.paused || document.hidden || !this.enabled) return;
    
    this.loading = true;
    this.error = null;
    
    try {
      const response = await fetch(url);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      this.data = await response.json();
    } catch (e) {
      this.error = e.message;
      console.warn('Auto-refresh failed:', e);
    } finally {
      this.loading = false;
    }
  },

  start() {
    if (this.interval) return;
    this.interval = setInterval(() => this.fetch(), intervalMs);
  },

  stop() {
    if (this.interval) {
      clearInterval(this.interval);
      this.interval = null;
    }
  },

  toggle() {
    this.enabled = !this.enabled;
    if (this.enabled) {
      this.fetch();
      this.start();
    } else {
      this.stop();
    }
  },

  handleVisibility() {
    if (document.hidden) {
      this.stop();
    } else if (this.enabled) {
      this.fetch();
      this.start();
    }
  },

  pause() {
    this.paused = true;
    this.stop();
  },

  resume() {
    this.paused = false;
    if (this.enabled && !document.hidden) {
      this.fetch();
      this.start();
    }
  }
});

/**
 * Network toggle component for VM details
 * Usage: x-data="networkToggle(index, initialEnabled, vmid, node, csrfToken)"
 */
window.networkToggle = (index, initialEnabled, vmid, node, csrfToken) => ({
  enabled: initialEnabled,
  loading: false,
  index: index,

  async toggle() {
    if (this.loading) return;
    
    this.loading = true;
    const newState = !this.enabled;
    
    try {
      const formData = new URLSearchParams({
        vmid: vmid,
        node: node,
        card_index: String(this.index),
        enabled: newState ? '1' : '0',
        csrf_token: csrfToken
      });

      const response = await fetch('/vm/toggle/network', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: formData
      });

      const result = await response.json();
      
      if (result.success) {
        this.enabled = newState;
        Alpine.store('notifications').add({
          type: 'success',
          message: newState ? this.$el.dataset.msgEnabled : this.$el.dataset.msgDisabled,
          duration: 3000
        });
      } else {
        Alpine.store('notifications').add({
          type: 'danger',
          message: result.error || this.$el.dataset.msgFailed,
          duration: 5000
        });
      }
    } catch (error) {
      console.error('Network toggle error:', error);
      Alpine.store('notifications').add({
        type: 'danger',
        message: this.$el.dataset.msgFailed,
        duration: 5000
      });
    } finally {
      this.loading = false;
    }
  }
});

/**
 * Memory converter component for VM creation
 * Usage: x-data="memoryConverter(minMB, maxMB, initialValue, selectedLabel)"
 */
window.memoryConverter = (minMB, maxMB, initialValue, selectedLabel) => ({
  value: initialValue || minMB,
  unit: 'MB',
  minMB: minMB,
  maxMB: maxMB,
  selectedLabel: selectedLabel || 'Selected',

  get minValue() {
    return this.unit === 'GB' ? (this.minMB / 1024).toFixed(1) : this.minMB;
  },

  get maxValue() {
    return this.unit === 'GB' ? Math.round(this.maxMB / 1024) : this.maxMB;
  },

  get step() {
    return this.unit === 'GB' ? '0.5' : '256';
  },

  get displayMB() {
    const val = parseFloat(String(this.value).replace(',', '.')) || 0;
    return this.unit === 'GB' ? Math.round(val * 1024) : Math.round(val);
  },

  get displayGB() {
    const val = parseFloat(String(this.value).replace(',', '.')) || 0;
    return this.unit === 'GB' ? val.toFixed(1) : (val / 1024).toFixed(1);
  },

  get displayText() {
    return `${this.selectedLabel}: ${this.displayGB} GB (${this.displayMB} MB)`;
  },

  changeUnit() {
    const currentVal = parseFloat(String(this.value).replace(',', '.')) || 0;
    if (this.unit === 'GB') {
      this.value = (currentVal / 1024).toFixed(1);
    } else {
      this.value = Math.round(currentVal * 1024);
    }
  }
});

/**
 * Admin login tabs component
 * Usage: x-data="adminLoginTabs()"
 */
window.adminLoginTabs = () => ({
  activeTab: 'local',

  setTab(tab) {
    this.activeTab = tab;
    this.$nextTick(() => {
      if (tab === 'local') {
        const el = document.getElementById('password');
        if (el) el.focus();
      } else {
        const el = document.getElementById('pve-username');
        if (el) el.focus();
      }
    });
  },

  addRealmIfMissing(event) {
    const input = event.target;
    let username = input.value.trim();
    if (username && !username.includes('@')) {
      input.value = username + '@pve';
    }
  },

  validatePveForm(event) {
    const usernameInput = document.getElementById('pve-username');
    let username = usernameInput ? usernameInput.value.trim() : '';
    if (username && !username.includes('@')) {
      usernameInput.value = username + '@pve';
    }
    if (!username) {
      event.preventDefault();
      alert('Please enter a username');
      return false;
    }
    const passwordInput = document.getElementById('pve-password');
    if (!passwordInput || !passwordInput.value) {
      event.preventDefault();
      alert('Please enter a password');
      return false;
    }
    return true;
  }
});
