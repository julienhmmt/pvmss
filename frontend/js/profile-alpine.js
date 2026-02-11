/**
 * Profile Page Alpine.js Component
 * Replaces profile.js with Alpine.js reactive components
 * Handles VM list auto-refresh, stats updates, and table rendering
 */

// Define component immediately for better compatibility
window.profilePage = () => ({
  vms: [],
  loading: false,
  error: null,
  refreshInterval: 45000,
  refreshTimer: null,
  
  // Stats
  get stats() {
    return {
      total: this.vms.length,
      running: this.vms.filter(vm => vm.status === 'running').length,
      stopped: this.vms.filter(vm => vm.status === 'stopped').length
    };
  },
  
  // Init
  init() {
    const card = document.getElementById('profile-vms-card');
    if (card && card.dataset.hasError !== '1') {
      const parsedInterval = parseInt(card.dataset.refreshInterval || '45000', 10);
      this.refreshInterval = Number.isFinite(parsedInterval) ? parsedInterval : 45000;
      
      this.fetchVMs();
      this.startAutoRefresh();
      
      // Handle visibility changes
      document.addEventListener('visibilitychange', () => this.handleVisibility());
    }
  },
    
    // Destroy
    destroy() {
      this.stopAutoRefresh();
    },
    
    // Fetch VMs
    async fetchVMs() {
      if (this.loading) return;
      
      this.loading = true;
      this.error = null;
      
      try {
        const response = await fetch('/api/profile/vms', {
          headers: { 'Cache-Control': 'no-cache' }
        });
        
        const data = await response.json();
        
        if (data.status === 'success') {
          this.vms = data.vms || [];
        } else {
          this.error = data.error || 'Failed to load VMs';
          this.vms = [];
        }
      } catch (e) {
        this.error = e.message || 'Network error';
        this.vms = [];
      } finally {
        this.loading = false;
      }
    },
    
    // Auto-refresh
    startAutoRefresh() {
      if (this.refreshTimer) return;
      this.refreshTimer = setInterval(() => this.fetchVMs(), this.refreshInterval);
    },
    
    stopAutoRefresh() {
      if (this.refreshTimer) {
        clearInterval(this.refreshTimer);
        this.refreshTimer = null;
      }
    },
    
    // Handle visibility
    handleVisibility() {
      if (document.hidden) {
        this.stopAutoRefresh();
      } else {
        this.fetchVMs();
        this.startAutoRefresh();
      }
    },
    
    // Manual refresh
    manualRefresh() {
      this.fetchVMs();
    },
    
    // Status helpers
    getStatusClass(status) {
      const s = (status || '').toLowerCase();
      if (s === 'running') return 'is-success';
      if (s === 'stopped') return 'is-danger';
      if (s === 'paused' || s === 'suspended') return 'is-warning';
      return 'is-light';
    },
    
    getStatusIcon(status) {
      const s = (status || '').toLowerCase();
      if (s === 'running') return 'fa-play';
      if (s === 'stopped') return 'fa-stop';
      if (s === 'paused' || s === 'suspended') return 'fa-pause';
      return 'fa-question';
    },
    
    // Format helpers
    formatMemory(bytes) {
      if (!bytes) return '0 MB';
      const gb = bytes / (1024 * 1024 * 1024);
      return gb >= 1 ? gb.toFixed(1) + ' GB' : (bytes / (1024 * 1024)).toFixed(0) + ' MB';
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
    
    // Simple markdown formatter for descriptions
    formatMarkdown(text) {
      if (!text) return '';
      
      return text
        // Bold: **text** -> <strong>text</strong>
        .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
        // Italic: *text* -> <em>text</em>
        .replace(/\*(.*?)\*/g, '<em>$1</em>')
        // Code: `code` -> <code>code</code>
        .replace(/`(.*?)`/g, '<code>$1</code>')
        // Line breaks: double newline -> <br>
        .replace(/\n\n/g, '<br><br>')
        // Single newline -> <br>
        .replace(/\n/g, '<br>');
    }
  });
