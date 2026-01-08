/**
 * Alpine.js initialization and global configuration - Optimized
 * This file must be loaded BEFORE Alpine.js
 */

document.addEventListener('alpine:init', () => {
  // Global notification store
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

  // Global loading store
  Alpine.store('loading', {
    states: {},

    set(key, value) {
      this.states[key] = value;
    },

    get(key) {
      return this.states[key] || false;
    },

    clear(key) {
      if (key) {
        delete this.states[key];
      } else {
        this.states = {};
      }
    }
  });

  // CSRF token helper
  Alpine.magic('csrf', () => {
    const meta = document.querySelector('meta[name="csrf-token"]');
    return meta ? meta.getAttribute('content') : '';
  });
});

/**
 * Dropdown component - Optimized
 * Usage: x-data="dropdown()"
 */
window.dropdown = () => ({
  open: false,

  toggle() {
    this.open = !this.open;
  },

  close() {
    this.open = false;
  }
});

/**
 * Dismissible notification component - Optimized
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
  }
});

/**
 * Tabs component - Optimized
 * Usage: x-data="tabs('default-tab')"
 */
window.tabs = (defaultTab = 'tab1') => ({
  activeTab: defaultTab,

  setTab(tabName) {
    this.activeTab = tabName;
  },

  isActive(tabName) {
    return this.activeTab === tabName;
  }
});

/**
 * Loading button component - Optimized
 * Usage: x-data="loadingButton()"
 */
window.loadingButton = () => ({
  loading: false,

  setLoading(state) {
    this.loading = state;
  },

  async submit(event) {
    if (this.loading) return;
    
    this.loading = true;
    try {
      // Form will submit normally
      await new Promise(resolve => setTimeout(resolve, 1000));
    } finally {
      this.loading = false;
    }
  }
});

/**
 * Modal component - Optimized
 * Usage: x-data="modal()"
 */
window.modal = () => ({
  open: false,

  show() {
    this.open = true;
    document.body.style.overflow = 'hidden';
  },

  hide() {
    this.open = false;
    document.body.style.overflow = '';
  }
});

/**
 * Memory converter component - Optimized
 * Usage: x-data="memoryConverter()"
 */
window.memoryConverter = () => ({
  value: 2048,
  unit: 'MB',
  minMB: 2048,
  maxMB: 65536,

  get minValue() {
    return this.unit === 'GB' ? this.minMB / 1024 : this.minMB;
  },

  get maxValue() {
    return this.unit === 'GB' ? this.maxMB / 1024 : this.maxMB;
  },

  get step() {
    return this.unit === 'GB' ? 0.5 : 256;
  },

  get displayText() {
    const mb = this.unit === 'GB' ? this.value * 1024 : this.value;
    const gb = (mb / 1024).toFixed(1);
    return `Selected: ${gb} GB (${Math.round(mb)} MB)`;
  },

  convertUnit() {
    if (this.unit === 'GB') {
      this.value = (this.value / 1024).toFixed(1);
    } else {
      this.value = Math.round(this.value * 1024);
    }
  }
});

/**
 * VM search component - Optimized
 * Usage: x-data="vmSearch()"
 */
window.vmSearch = () => ({
  query: '',
  status: 'all',

  filter(vms) {
    return vms.filter(vm => {
      const matchesQuery = !this.query || 
        vm.name?.toLowerCase().includes(this.query.toLowerCase()) ||
        String(vm.vmid).includes(this.query);
      
      const matchesStatus = this.status === 'all' || vm.status === this.status;
      
      return matchesQuery && matchesStatus;
    });
  },

  reset() {
    this.query = '';
    this.status = 'all';
  }
});

/**
 * Auto-refresh component - Optimized
 * Usage: x-data="autoRefresh(url, interval)"
 */
window.autoRefresh = (url, interval = 30000) => ({
  data: null,
  loading: false,
  error: null,
  intervalId: null,
  paused: false,

  async fetch() {
    if (this.paused || document.hidden) return;
    
    this.loading = true;
    this.error = null;
    
    try {
      const response = await fetch(url);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      this.data = await response.json();
    } catch (error) {
      this.error = error.message;
      console.warn('Auto-refresh failed:', error);
    } finally {
      this.loading = false;
    }
  },

  start() {
    this.fetch();
    this.intervalId = setInterval(() => this.fetch(), interval);
  },

  stop() {
    if (this.intervalId) {
      clearInterval(this.intervalId);
      this.intervalId = null;
    }
  },

  handleVisibility() {
    if (document.hidden) {
      this.stop();
    } else {
      this.start();
    }
  }
});

/**
 * Network toggle component - Optimized
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
      Alpine.store('notifications').add({
        type: 'danger',
        message: this.$el.dataset.msgError || 'Network toggle failed',
        duration: 5000
      });
    } finally {
      this.loading = false;
    }
  },

  get statusText() {
    if (this.loading) return 'Processing...';
    return this.enabled ? 'Enabled' : 'Disabled';
  },

  get statusClass() {
    return this.enabled ? 'has-text-success' : 'has-text-grey';
  }
});

/**
 * Admin login tabs component - Optimized
 * Usage: x-data="adminLoginTabs()"
 */
window.adminLoginTabs = () => ({
  activeTab: 'local',

  setTab(tabName) {
    this.activeTab = tabName;
  },

  isActive(tabName) {
    return this.activeTab === tabName;
  }
});
