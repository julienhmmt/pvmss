/**
 * VM Utilities - Optimized shared functions for VM-related pages
 * Used by: Alpine.js components and legacy pages
 */

// Utility functions
const VMUtils = {
  // Format bytes to human-readable string
  formatBytes(bytes, decimals = 2) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + ' ' + sizes[i];
  },

  // Format uptime seconds to human-readable string
  formatUptime(seconds) {
    if (!seconds || seconds <= 0) return '--';
    
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    
    const parts = [];
    if (days > 0) parts.push(days + 'd');
    if (hours > 0) parts.push(hours + 'h');
    if (minutes > 0 || parts.length === 0) parts.push(minutes + 'm');
    
    return parts.join(' ');
  },

  // Parse tags string into array
  parseTags(tagsString, excludeTags = ['pvmss']) {
    if (!tagsString) return [];
    return tagsString
      .split(/[,;]/)
      .map(tag => tag.trim())
      .filter(tag => tag && !excludeTags.includes(tag));
  },

  // Escape HTML to prevent XSS
  escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  },

  // Get CSRF token from meta
  getCSRFToken() {
    const meta = document.querySelector('meta[name="csrf-token"]');
    return meta ? meta.getAttribute('content') : '';
  },

  // Show notification using Alpine.js store
  showNotification(type, message, duration = 5000) {
    if (window.Alpine && Alpine.store) {
      Alpine.store('notifications').add({ type, message, duration });
    } else {
      // Fallback for non-Alpine pages
      console.log(`[${type.toUpperCase()}] ${message}`);
    }
  },

  // Handle API errors consistently
  handleApiError(error, context = 'API request') {
    console.error(`${context} failed:`, error);
    let message = 'An unexpected error occurred';
    
    if (error.response) {
      const status = error.response.status;
      if (status === 403) message = 'Access denied';
      else if (status === 404) message = 'Resource not found';
      else if (status >= 500) message = 'Server error occurred';
    } else if (error.name === 'TimeoutError') {
      message = 'Request timed out';
    } else if (error.name === 'NetworkError') {
      message = 'Network connection failed';
    }
    
    this.showNotification('danger', message);
  },

  // Debounce function for search/filter
  debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
      const later = () => {
        clearTimeout(timeout);
        func(...args);
      };
      clearTimeout(timeout);
      timeout = setTimeout(later, wait);
    };
  }
};

// Export for global use
window.VMUtils = VMUtils;
