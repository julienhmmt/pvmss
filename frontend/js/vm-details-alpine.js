/**
 * VM Details Page Alpine.js Component
 * Replaces vm-details.js with Alpine.js reactive components
 * Handles network card toggles, auto-refresh metrics, skeletons,
 * local notifications, celebration banner actions, and VM action loaders.
 */

// Define component immediately for better compatibility
window.vmDetails = () => ({
    vmid: '',
    node: '',
    status: '',
    metrics: null,
    loading: false,
    error: null,
    refreshInterval: 30000,
    refreshTimer: null,
    showCelebration: false,
    
    // Init
    init() {
      const configEl = document.getElementById('vm-details-config');
      const cfg = configEl ? configEl.dataset : {};
      
      this.vmid = cfg.vmid || '';
      this.node = cfg.node || '';
      this.status = cfg.status || '';
      
      // Show celebration banner if VM was just created
      if (window.location.search.includes('created=1')) {
        this.showCelebration = true;
        // Hide after 10 seconds
        setTimeout(() => {
          this.showCelebration = false;
        }, 10000);
      }
      
      // Show flash messages
      this.showFlashMessages();
      
      // Start auto-refresh for metrics
      this.startMetricsRefresh();
      
      // Handle visibility changes
      document.addEventListener('visibilitychange', () => this.handleVisibility());
    },
    
    // Destroy
    destroy() {
      this.stopMetricsRefresh();
    },
    
    // Flash messages
    showFlashMessages() {
      const configEl = document.getElementById('vm-details-config');
      const cfg = configEl ? configEl.dataset : {};
      
      const errorMsg = cfg.errorMessage || this.readQueryParam('error_msg');
      const warningMsg = cfg.warningMessage || this.readQueryParam('warning_msg');
      const successMsg = cfg.successMessage || this.readQueryParam('success_msg');
      
      // Clear query params
      if (errorMsg || warningMsg || successMsg) {
        const params = new URLSearchParams(window.location.search);
        params.delete('error');
        params.delete('error_msg');
        params.delete('warning');
        params.delete('warning_msg');
        params.delete('success');
        params.delete('success_msg');
        const cleanSearch = params.toString();
        const newUrl = cleanSearch ? `${window.location.pathname}?${cleanSearch}` : window.location.pathname;
        window.history.replaceState({}, '', newUrl);
      }
      
      // Show notifications
      if (errorMsg) {
        Alpine.store('notifications').add({
          type: 'danger',
          message: errorMsg,
          duration: 8000
        });
      }
      
      if (warningMsg) {
        Alpine.store('notifications').add({
          type: 'warning',
          message: warningMsg,
          duration: 6000
        });
      }
      
      if (successMsg) {
        Alpine.store('notifications').add({
          type: 'success',
          message: successMsg,
          duration: 5000
        });
      }
    },
    
    readQueryParam(name) {
      const params = new URLSearchParams(window.location.search);
      const value = params.get(name);
      return value ? value.trim() : '';
    },
    
    // Metrics refresh
    startMetricsRefresh() {
      if (this.refreshTimer) return;
      this.refreshTimer = setInterval(() => this.fetchMetrics(), this.refreshInterval);
    },
    
    stopMetricsRefresh() {
      if (this.refreshTimer) {
        clearInterval(this.refreshTimer);
        this.refreshTimer = null;
      }
    },
    
    async fetchMetrics() {
      if (this.loading || document.hidden) return;
      
      this.loading = true;
      this.error = null;
      
      try {
        const response = await fetch(`/api/vm/${this.vmid}/metrics`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        this.metrics = await response.json();
      } catch (e) {
        this.error = e.message;
        console.warn('Metrics refresh failed:', e);
      } finally {
        this.loading = false;
      }
    },
    
    // Handle visibility
    handleVisibility() {
      if (document.hidden) {
        this.stopMetricsRefresh();
      } else {
        this.fetchMetrics();
        this.startMetricsRefresh();
      }
    },
    
    // Console actions
    openConsole() {
      const statusBadge = document.getElementById('vm-status-badge');
      const status = statusBadge ? statusBadge.dataset.status || 'stopped' : 'stopped';
      
      if (status !== 'running') {
        Alpine.store('notifications').add({
          type: 'warning',
          message: this.$t('consoleMustBeRunning') || 'VM must be running to open console. Please start the VM first.',
          duration: 5000
        });
        return;
      }
      
      // Open console in new window
      const consoleUrl = `/vm/console?vmid=${this.vmid}&node=${this.node}`;
      const consoleWindow = window.open(consoleUrl, '_blank', 'width=1024,height=768,resizable=yes,scrollbars=yes');
      
      if (consoleWindow) {
        consoleWindow.focus();
      } else {
        Alpine.store('notifications').add({
          type: 'danger',
          message: this.$t('consoleUnavailable') || 'Console is temporarily unavailable. Please try again in a moment.',
          duration: 5000
        });
      }
    },
    
    // Celebration actions
    createAnotherVM() {
      window.location.href = '/vm/create';
    },
    
    viewVM() {
      // Already on VM details page, just hide celebration
      this.showCelebration = false;
    },
    
    // Format helpers
    formatMemory(bytes) {
      if (!bytes) return '0 MB';
      const gb = bytes / (1024 * 1024 * 1024);
      return gb >= 1 ? gb.toFixed(1) + ' GB' : (bytes / (1024 * 1024)).toFixed(0) + ' MB';
    },
    
    formatMemoryGB(valueMB) {
      const gb = valueMB / 1024;
      return gb >= 1 ? gb.toFixed(1) + ' GB' : valueMB + ' MB';
    },
    
    formatCPU(cores) {
      if (!cores) return '0';
      return cores === 1 ? '1 vCPU' : `${cores} vCPUs`;
    },
    
    formatDisk(bytes) {
      if (!bytes) return '0 GB';
      const gb = bytes / (1024 * 1024 * 1024);
      const tb = gb / 1024;
      
      if (tb >= 1) {
        return tb.toFixed(1) + ' TB';
      } else if (gb >= 1) {
        return gb.toFixed(1) + ' GB';
      } else {
        return (bytes / (1024 * 1024)).toFixed(0) + ' MB';
      }
    },
    
    formatUptime(seconds) {
      if (!seconds) return '0s';
      const days = Math.floor(seconds / 86400);
      const hours = Math.floor((seconds % 86400) / 3600);
      const minutes = Math.floor((seconds % 3600) / 60);
      
      if (days > 0) return `${days}d ${hours}h`;
      if (hours > 0) return `${hours}h ${minutes}m`;
      return `${minutes}m`;
    },
    
    formatNetwork(bps) {
      if (!bps) return '0 Mbps';
      const mbps = bps / 1000000;
      const gbps = mbps / 1000;
      
      if (gbps >= 1) {
        return gbps.toFixed(1) + ' Gbps';
      } else if (mbps >= 1) {
        return mbps.toFixed(1) + ' Mbps';
      } else {
        return (bps / 1000).toFixed(0) + ' Kbps';
      }
    }
  });
