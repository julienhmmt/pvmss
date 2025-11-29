/**
 * VM Utilities - Shared functions for VM-related pages
 * Used by: profile.js, search-page.js, vm-details.js
 */

const VMUtils = (function() {
  'use strict';

  /**
   * Status configuration with CSS classes and icons
   */
  const STATUS_CONFIG = {
    running: { className: 'is-success', icon: 'fa-circle-play', label: 'Running' },
    stopped: { className: 'is-danger', icon: 'fa-circle-stop', label: 'Stopped' },
    paused: { className: 'is-warning', icon: 'fa-circle-pause', label: 'Paused' },
    suspended: { className: 'is-warning', icon: 'fa-circle-pause', label: 'Suspended' },
    unknown: { className: 'is-warning', icon: 'fa-circle-question', label: 'Unknown' }
  };

  /**
   * Get status configuration for a VM status
   * @param {string} status - VM status (running, stopped, etc.)
   * @param {Object} translations - Optional translations object with status labels
   * @returns {Object} Status config with className, icon, and label
   */
  function getStatusConfig(status, translations) {
    const normalized = (status || '').toLowerCase();
    const config = STATUS_CONFIG[normalized] || STATUS_CONFIG.unknown;
    
    // Apply translations if provided
    if (translations) {
      const translationKey = 'status' + normalized.charAt(0).toUpperCase() + normalized.slice(1);
      if (translations[translationKey]) {
        return { ...config, label: translations[translationKey] };
      }
    }
    
    return config;
  }

  /**
   * Get CSS class for status badge
   * @param {string} status - VM status
   * @returns {string} Bulma CSS class
   */
  function getStatusBadgeClass(status) {
    return getStatusConfig(status).className;
  }

  /**
   * Get Font Awesome icon class for status
   * @param {string} status - VM status
   * @returns {string} FA icon class (without 'fas' prefix)
   */
  function getStatusIcon(status) {
    return getStatusConfig(status).icon;
  }

  /**
   * Escape HTML to prevent XSS
   * @param {string} text - Text to escape
   * @returns {string} Escaped text
   */
  function escapeHtml(text) {
    if (text === null || text === undefined) return '';
    const div = document.createElement('div');
    div.textContent = String(text);
    return div.innerHTML;
  }

  /**
   * Create a Font Awesome icon element
   * @param {string} iconClass - Icon class (e.g., 'fa-server')
   * @param {string} prefix - Icon prefix (default: 'fas')
   * @returns {HTMLElement} Icon element
   */
  function createIcon(iconClass, prefix) {
    const icon = document.createElement('i');
    icon.className = `${prefix || 'fas'} ${iconClass}`;
    return icon;
  }

  /**
   * Create a status badge element
   * @param {string} status - VM status
   * @param {Object} translations - Optional translations
   * @returns {HTMLElement} Badge element
   */
  function createStatusBadge(status, translations) {
    const config = getStatusConfig(status, translations);
    const badge = document.createElement('span');
    badge.className = `tag ${config.className}`;

    const iconSpan = document.createElement('span');
    iconSpan.className = 'icon';
    iconSpan.appendChild(createIcon(config.icon));

    const textSpan = document.createElement('span');
    textSpan.textContent = config.label;

    badge.append(iconSpan, textSpan);
    return badge;
  }

  /**
   * Create a placeholder element for empty values
   * @param {string} text - Placeholder text
   * @returns {HTMLElement} Placeholder element
   */
  function createPlaceholder(text) {
    const placeholder = document.createElement('em');
    placeholder.className = 'has-text-grey-light';
    placeholder.textContent = text || '';
    return placeholder;
  }

  /**
   * Toggle hidden class on element
   * @param {HTMLElement} element - Target element
   * @param {boolean} hide - Whether to hide
   */
  function toggleHidden(element, hide) {
    if (!element) return;
    element.classList.toggle('is-hidden', !!hide);
  }

  /**
   * Format bytes to human-readable string
   * @param {number} bytes - Size in bytes
   * @param {number} decimals - Decimal places (default: 2)
   * @returns {string} Formatted string (e.g., "1.5 GB")
   */
  function formatBytes(bytes, decimals) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const dm = decimals || 2;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
  }

  /**
   * Format uptime seconds to human-readable string
   * @param {number} seconds - Uptime in seconds
   * @returns {string} Formatted string (e.g., "2d 5h 30m")
   */
  function formatUptime(seconds) {
    if (!seconds || seconds <= 0) return '--';
    
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    
    const parts = [];
    if (days > 0) parts.push(days + 'd');
    if (hours > 0) parts.push(hours + 'h');
    if (minutes > 0 || parts.length === 0) parts.push(minutes + 'm');
    
    return parts.join(' ');
  }

  /**
   * Parse tags string into array
   * @param {string} tagsString - Comma or semicolon separated tags
   * @param {Array} excludeTags - Tags to exclude (default: ['pvmss'])
   * @returns {Array} Array of tag strings
   */
  function parseTags(tagsString, excludeTags) {
    if (!tagsString) return [];
    const exclude = excludeTags || ['pvmss'];
    return tagsString
      .split(/[,;]/)
      .map(tag => tag.trim())
      .filter(tag => tag && !exclude.includes(tag));
  }

  /**
   * Create tag elements from tags string
   * @param {string} tagsString - Comma separated tags
   * @param {string} emptyText - Text to show if no tags
   * @returns {HTMLElement} Container with tags or placeholder
   */
  function createTagElements(tagsString, emptyText) {
    const container = document.createElement('span');
    const tags = parseTags(tagsString);
    
    if (tags.length === 0) {
      container.appendChild(createPlaceholder(emptyText || 'No tags'));
      return container;
    }
    
    tags.forEach((tag, index) => {
      const tagEl = document.createElement('span');
      tagEl.className = 'tag is-small is-info is-light';
      tagEl.textContent = escapeHtml(tag);
      container.appendChild(tagEl);
      if (index < tags.length - 1) {
        container.appendChild(document.createTextNode(' '));
      }
    });
    
    return container;
  }

  /**
   * Debounce function calls
   * @param {Function} func - Function to debounce
   * @param {number} wait - Wait time in ms
   * @returns {Function} Debounced function
   */
  function debounce(func, wait) {
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

  /**
   * Fetch JSON with error handling and CSRF support
   * @param {string} url - URL to fetch
   * @param {Object} options - Fetch options
   * @returns {Promise<Object>} Response with success, data, and error properties
   */
  async function fetchJson(url, options = {}) {
    const defaultOptions = {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json'
      },
      credentials: 'same-origin'
    };

    // Add CSRF token for non-GET requests
    if (options.method && options.method.toUpperCase() !== 'GET') {
      const csrfToken = getCSRFToken();
      if (csrfToken) {
        defaultOptions.headers['X-CSRF-Token'] = csrfToken;
      }
    }

    const finalOptions = {
      ...defaultOptions,
      ...options,
      headers: {
        ...defaultOptions.headers,
        ...options.headers
      }
    };

    try {
      const response = await fetch(url, finalOptions);
      
      if (!response.ok) {
        const errorText = await response.text();
        console.error(`fetchJson error: ${response.status} ${response.statusText}`, errorText);
        return {
          success: false,
          error: `HTTP ${response.status}: ${response.statusText}`,
          data: null
        };
      }

      const data = await response.json();
      return {
        success: true,
        error: null,
        data: data
      };
    } catch (error) {
      console.error('fetchJson network error:', error);
      return {
        success: false,
        error: error.message || 'Network error',
        data: null
      };
    }
  }

  /**
   * Get CSRF token from meta tag or global helper
   * @returns {string|null} CSRF token or null if not found
   */
  function getCSRFToken() {
    // Prefer centralized helper when available
    if (window.PVMSS && window.PVMSS.security && typeof window.PVMSS.security.getCSRFTokenFromMeta === 'function') {
      return window.PVMSS.security.getCSRFTokenFromMeta();
    }
    const meta = document.querySelector('meta[name="csrf-token"]');
    return meta ? meta.getAttribute('content') : null;
  }

  // Public API
  return {
    STATUS_CONFIG,
    getStatusConfig,
    getStatusBadgeClass,
    getStatusIcon,
    escapeHtml,
    createIcon,
    createStatusBadge,
    createPlaceholder,
    toggleHidden,
    formatBytes,
    formatUptime,
    parseTags,
    createTagElements,
    debounce,
    fetchJson,
    getCSRFToken
  };
})();

// Export for ES modules if supported
if (typeof module !== 'undefined' && module.exports) {
  module.exports = VMUtils;
}
