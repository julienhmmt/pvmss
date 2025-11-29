/**
 * VM Details Page JavaScript Module
 * Handles network card toggles, auto-refresh metrics, skeletons,
 * local notifications, celebration banner actions, and VM action loaders.
 */

(function () {
  'use strict';

  // Inject celebration and card hover styles
  const style = document.createElement('style');
  style.textContent = '@keyframes slideInBounce{0%{opacity:0;transform:translateY(-20px) scale(0.95)}50%{opacity:0.8;transform:translateY(5px) scale(1.02)}100%{opacity:1;transform:translateY(0) scale(1)}}@keyframes slideOut{0%{opacity:1;transform:translateY(0) scale(1)}100%{opacity:0;transform:translateY(-10px) scale(0.95)}}#vm-celebration-banner{box-shadow:0 8px 25px rgba(72,199,116,0.2);border-radius:12px}#vm-celebration-banner .button{transition:all 0.2s ease}#vm-celebration-banner .button:hover{transform:translateY(-1px);box-shadow:0 4px 12px rgba(0,0,0,0.15)}.stat-card:hover,.network-card:hover,.security-feature-card:hover,.network-interface-card:hover,.disk-item:hover,.cdrom-item:hover{box-shadow:0 4px 12px rgba(0,0,0,0.08)!important}';
  document.head.appendChild(style);

  // Config from DOM
  const configEl = document.getElementById('vm-details-config');
  const cfg = configEl ? configEl.dataset : {};
  const processingLabel = cfg.processingLabel || 'Processing';
  const msgNetworkEnabled = cfg.msgNetworkEnabled || 'Network card enabled';
  const msgNetworkDisabled = cfg.msgNetworkDisabled || 'Network card disabled';
  const msgActionFailed = cfg.msgActionFailed || 'Action failed';
  const labelEnabled = cfg.labelEnabled || 'Enabled';
  const labelDisabled = cfg.labelDisabled || 'Disabled';
  const labelEnable = cfg.labelEnable || 'Enable';
  const labelDisable = cfg.labelDisable || 'Disable';
  const networkLabel = cfg.networkLabel || 'Network';
  const vmid = cfg.vmid || '';
  const node = cfg.node || '';

  // Network card toggle function - Enhanced for instant feedback
  function toggleNetworkCard(cardIndex, enabled, csrfToken, vmidArg, nodeArg) {
    const toggleContainer = document.querySelector('#network-toggle-' + cardIndex);
    const toggleLabel = document.querySelector('#network-toggle-' + cardIndex + ' .network-toggle-label');
    const toggleInput = document.querySelector('#network-toggle-' + cardIndex + ' input[type="checkbox"]');

    if (!toggleContainer || !toggleLabel || !toggleInput) {
      return;
    }

    // Show loading state immediately
    const originalLabel = toggleLabel.innerHTML;
    toggleContainer.classList.add('loading');
    toggleLabel.innerHTML = '<i class="fas fa-spinner fa-spin"></i> ' + processingLabel + '...';
    toggleInput.disabled = true;

    const formData = new URLSearchParams();
    formData.append('csrf_token', csrfToken);
    formData.append('vmid', vmidArg);
    formData.append('node', nodeArg);
    formData.append('card_index', String(cardIndex));
    formData.append('enabled', enabled ? '1' : '0');

    fetch('/vm/toggle/network', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded'
      },
      body: formData
    })
      .then((response) => {
        if (response.redirected) {
          window.location.href = response.url;
          return;
        }

        toggleContainer.classList.remove('loading');

        if (response.ok) {
          // Add success animation
          toggleContainer.classList.add('success');
          setTimeout(() => toggleContainer.classList.remove('success'), 600);

          // Update UI immediately for instant feedback
          updateNetworkCardUI(cardIndex, enabled);

          // Show success notification
          const successMsg = enabled ? msgNetworkEnabled : msgNetworkDisabled;
          showSuccess(successMsg);

          // Optional: Refresh data in background after a short delay
          setTimeout(() => {
            refreshNetworkData();
          }, 1000);
        } else {
          // Add error animation
          toggleContainer.classList.add('error');
          setTimeout(() => toggleContainer.classList.remove('error'), 500);

          // Revert UI on failure
          toggleInput.checked = !enabled;
          toggleLabel.innerHTML = originalLabel;
          toggleInput.disabled = false;

          console.error('Network toggle failed');
          showError(msgActionFailed);
        }
      })
      .catch((error) => {
        toggleContainer.classList.remove('loading');

        // Add error animation
        toggleContainer.classList.add('error');
        setTimeout(() => toggleContainer.classList.remove('error'), 500);

        // Revert UI on error
        toggleInput.checked = !enabled;
        toggleLabel.innerHTML = originalLabel;
        toggleInput.disabled = false;

        console.error('Error:', error);
        showError(msgActionFailed);
      });
  }

  // Update network card UI instantly
  function updateNetworkCardUI(cardIndex, enabled) {
    const toggleLabel = document.querySelector('#network-toggle-' + cardIndex + ' .network-toggle-label');
    const toggleInput = document.querySelector('#network-toggle-' + cardIndex + ' input[type="checkbox"]');
    const toggleContainer = document.querySelector('#network-toggle-' + cardIndex);

    if (!toggleLabel || !toggleInput || !toggleContainer) {
      return;
    }

    // Update checkbox state and ARIA
    toggleInput.checked = enabled;
    toggleInput.setAttribute('aria-checked', enabled ? 'true' : 'false');

    // Update label with smooth transition
    toggleLabel.style.opacity = '0';
    setTimeout(() => {
      if (enabled) {
        toggleLabel.innerHTML = '<i class="fas fa-plug-circle-check has-text-success"></i> ' + labelEnabled;
      } else {
        toggleLabel.innerHTML = '<i class="fas fa-plug-circle-xmark has-text-grey"></i> ' + labelDisabled;
      }
      toggleLabel.style.opacity = '1';
    }, 100);

    // Re-enable controls
    toggleInput.disabled = false;

    // Update tooltip
    const actionLabel = enabled ? labelDisable : labelEnable;
    const index = parseInt(String(cardIndex), 10) + 1;
    toggleContainer.title = actionLabel + ' ' + networkLabel + ' #' + index;
  }

  // Refresh network data in background (optional)
  function refreshNetworkData() {
    // This could refresh network interface data without full page reload
    // For now, we'll skip it to keep the experience fast
    console.log('Network data refreshed');
  }

  // Auto-refresh VM metrics with visibility optimization
  let autoRefreshInterval = null;
  let isAutoRefreshEnabled = true;
  let isPageVisible = true;

  // Visibility API to pause auto-refresh when page is not visible
  function handleVisibilityChange() {
    isPageVisible = !document.hidden;
    if (isPageVisible) {
      startAutoRefresh();
    } else if (autoRefreshInterval) {
      clearInterval(autoRefreshInterval);
      autoRefreshInterval = null;
    }
  }

  if (typeof document.hidden !== 'undefined') {
    document.addEventListener('visibilitychange', handleVisibilityChange);
  }

  const statusBadge = document.getElementById('vm-status-badge');
  const cpuProgress = document.querySelector('[data-progress="cpu"]');
  const ramProgress = document.querySelector('[data-progress="ram"]');
  const cpuMetric = document.querySelector('[data-metric="cpu"]');
  const ramMetric = document.querySelector('[data-metric="ram"]');
  const uptimeDisplay = document.querySelector('[data-uptime]');
  let lastStatus = statusBadge ? statusBadge.dataset.status || '' : '';

  function startAutoRefresh() {
    if (autoRefreshInterval) {
      clearInterval(autoRefreshInterval);
    }

    if (!isPageVisible || !isAutoRefreshEnabled || !vmid) {
      return;
    }

    autoRefreshInterval = setInterval(() => {
      if (!isAutoRefreshEnabled || !isPageVisible) {
        return;
      }

      fetch('/api/vm/' + vmid + '/metrics')
        .then((response) => {
          if (!response.ok) throw new Error('Failed to fetch metrics');
          return response.json();
        })
        .then((data) => {
          updateVMMetrics(data);
        })
        .catch((error) => {
          console.log('Auto-refresh paused:', error.message);
          setTimeout(() => {
            if (isPageVisible && isAutoRefreshEnabled) {
              startAutoRefresh();
            }
          }, 10000);
        });
    }, 30000);
  }

  function showSkeleton(cardId) {
    const card = document.getElementById(cardId);
    if (!card) return;

    const content = card.querySelector('.card-content');
    if (!content) return;

    card.classList.add('is-loading');
    const realContent = content.innerHTML;
    content.setAttribute('data-original-content', realContent);

    content.innerHTML = `
      <div class="is-flex is-align-items-center is-justify-content-space-between mb-4">
        <div class="is-flex is-align-items-center">
          <span class="icon skeleton skeleton-circle mr-3"></span>
          <div>
            <div class="skeleton skeleton-title"></div>
            <div class="skeleton skeleton-text" style="width: 80%;"></div>
          </div>
        </div>
        <div class="has-text-right">
          <div class="skeleton skeleton-title" style="width: 50px;"></div>
          <div class="skeleton skeleton-text" style="width: 60px;"></div>
        </div>
      </div>
      <div style="margin-top: 1.5rem;">
        <div class="skeleton skeleton-text" style="width: 90%;"></div>
        <div class="skeleton skeleton-text" style="width: 70%; margin-top: 0.5rem;"></div>
        <div style="margin-top: 0.75rem;">
          <div class="skeleton" style="height: 6px; border-radius: 3px;"></div>
        </div>
      </div>
    `;
  }

  function hideSkeleton(cardId) {
    const card = document.getElementById(cardId);
    if (!card) return;

    const content = card.querySelector('.card-content');
    if (!content) return;

    const originalContent = content.getAttribute('data-original-content');
    if (originalContent) {
      content.innerHTML = originalContent;
      card.classList.remove('is-loading');
      card.classList.add('fade-in');
      setTimeout(() => {
        card.classList.remove('fade-in');
      }, 300);
    }
  }

  function formatBytes(bytes) {
    const gb = bytes / (1024 * 1024 * 1024);
    if (gb >= 1) {
      return gb.toFixed(1) + ' GB';
    }
    const mb = bytes / (1024 * 1024);
    return mb.toFixed(0) + ' MB';
  }

  function formatUptime(seconds) {
    if (seconds < 60) return seconds + 's';
    if (seconds < 3600) return Math.floor(seconds / 60) + 'm';
    if (seconds < 86400) return Math.floor(seconds / 3600) + 'h';
    return Math.floor(seconds / 86400) + 'd';
  }

  function updateStatusBadge(status) {
    if (!statusBadge) return;

    const el = statusBadge.querySelector('.tag');
    if (!el) return;

    el.classList.remove('is-success', 'is-danger', 'is-warning', 'is-info');

    let className = 'is-info';
    switch ((status || '').toLowerCase()) {
      case 'running':
        className = 'is-success';
        break;
      case 'stopped':
        className = 'is-danger';
        break;
      case 'paused':
      case 'suspended':
        className = 'is-warning';
        break;
      default:
        className = 'is-warning';
        break;
    }

    el.classList.add(className);
  }

  function updateVMMetrics(data) {
    const status = data.status;
    const cpuPercent = data.cpu ? Math.round(data.cpu * 100) : 0;
    const memUsed = data.mem || 0;
    const memTotal = data.maxmem || 1;
    const memPercent = Math.round((memUsed / memTotal) * 100);

    if (status !== lastStatus) {
      updateStatusBadge(status);
      lastStatus = status;

      const isRunning = status === 'running';
      if (cpuProgress) cpuProgress.style.display = isRunning ? '' : 'none';
      if (ramProgress) ramProgress.style.display = isRunning ? '' : 'none';

      if (cpuMetric) {
        cpuMetric.textContent = isRunning ? cpuPercent + '%' : '--';
      }
      if (ramMetric) {
        ramMetric.textContent = isRunning ? formatBytes(memUsed) : '--';
      }
    }

    if (status === 'running') {
      if (cpuMetric) cpuMetric.textContent = cpuPercent + '%';
      if (cpuProgress) cpuProgress.value = cpuPercent;

      if (ramMetric) ramMetric.textContent = formatBytes(memUsed);
      if (ramProgress) {
        ramProgress.value = memUsed;
        ramProgress.max = memTotal;
      }
    }

    if (data.uptime !== undefined && uptimeDisplay) {
      uptimeDisplay.textContent = formatUptime(data.uptime);
    }
  }

  // Local notification system (for this page only)
  function showNotification(message, type, autoDismiss) {
    const notificationId = 'notification-' + Date.now();
    const notification = document.createElement('div');
    notification.id = notificationId;
    notification.className = 'notification is-' + (type || 'info') + ' is-light';
    notification.style.cssText = [
      'position: fixed',
      'top: 20px',
      'right: 20px',
      'z-index: 9999',
      'max-width: 350px',
      'min-width: 250px',
      'opacity: 0',
      'transform: translateX(100%)',
      'transition: all 0.3s ease',
      'box-shadow: 0 4px 12px rgba(0,0,0,0.15)'
    ].join('; ');

    let icon = 'fas fa-info-circle';
    if (type === 'success') icon = 'fas fa-check-circle';
    if (type === 'warning') icon = 'fas fa-exclamation-triangle';
    if (type === 'danger') icon = 'fas fa-times-circle';

    notification.innerHTML = [
      '<button class="delete" aria-label="Close notification"></button>',
      '<div class="media">',
      '  <div class="media-left">',
      '    <span class="icon has-text-' + type + '">',
      '      <i class="' + icon + '"></i>',
      '    </span>',
      '  </div>',
      '  <div class="media-content">',
      '    <p>' + message + '</p>',
      '  </div>',
      '</div>'
    ].join('');

    document.body.appendChild(notification);

    positionNotification(notification);

    setTimeout(() => {
      notification.style.opacity = '1';
      notification.style.transform = 'translateX(0)';
    }, 50);

    if (autoDismiss) {
      const dismissTime = type === 'success' ? 3000 : type === 'danger' ? 8000 : 5000;
      setTimeout(() => {
        dismissNotification(notificationId);
      }, dismissTime);
    }

    const closeBtn = notification.querySelector('.delete');
    if (closeBtn) {
      closeBtn.addEventListener('click', () => dismissNotification(notificationId));
    }

    return notificationId;
  }

  function dismissNotification(notificationId) {
    const notification = document.getElementById(notificationId);
    if (!notification) return;

    notification.style.opacity = '0';
    notification.style.transform = 'translateX(100%)';

    setTimeout(() => {
      if (notification.parentElement) {
        notification.remove();
        repositionAllNotifications();
      }
    }, 300);
  }

  function positionNotification(notification) {
    const existingNotifications = document.querySelectorAll('.notification[id^="notification-"]:not([id="' + notification.id + '"])');
    const topOffset = 20 + existingNotifications.length * 80;
    notification.style.top = topOffset + 'px';
  }

  function repositionAllNotifications() {
    const notifications = document.querySelectorAll('.notification[id^="notification-"]');
    notifications.forEach((notif, index) => {
      const topOffset = 20 + index * 80;
      notif.style.top = topOffset + 'px';
    });
  }

  function showSuccess(message) {
    showNotification(message, 'success', true);
  }

  function showError(message) {
    showNotification(message, 'danger', true);
  }

  // Celebration banner functions
  function dismissCelebration() {
    const banner = document.getElementById('vm-celebration-banner');
    if (banner) {
      banner.style.animation = 'slideOut 0.3s ease-in forwards';
      setTimeout(() => {
        banner.remove();
      }, 300);
    }
  }

  function startVM() {
    const startBtn = document.querySelector('form[action="/vm/action"] input[value="start"]');
    if (startBtn) {
      const form = startBtn.closest('form');
      if (form) {
        form.submit();
      }
    } else {
      showError('Start button not found');
    }
  }

  function openConsoleShortcut() {
    const consoleBtn = document.querySelector('a[href*="console"]');
    if (consoleBtn) {
      window.open(consoleBtn.href, '_blank');
    } else {
      showError('Console link not found');
    }
  }

  function editResources() {
    const editBtn = document.querySelector('a[href*="edit=resources"]');
    if (editBtn) {
      window.location.href = editBtn.href;
    } else {
      showError('Resources edit link not found');
    }
  }

  function pauseAutoRefresh() {
    isAutoRefreshEnabled = false;
    setTimeout(() => {
      isAutoRefreshEnabled = true;
    }, 60000);
  }

  function showLoadingOnButton(button, loadingText) {
    if (!button) return;

    const originalContent = button.innerHTML;
    button.setAttribute('data-original-content', originalContent);

    button.innerHTML = [
      '<span class="icon">',
      '  <i class="fas fa-spinner fa-spin"></i>',
      '</span>',
      '<span>' + loadingText + '</span>'
    ].join('');
    button.disabled = true;
    button.classList.add('is-loading');
  }

  function restoreButton(button) {
    if (!button) return;

    const originalContent = button.getAttribute('data-original-content');
    if (originalContent) {
      button.innerHTML = originalContent;
      button.removeAttribute('data-original-content');
    }
    button.disabled = false;
    button.classList.remove('is-loading');
  }

  function enhanceVMActionForms() {
    const actionForms = document.querySelectorAll('form[action="/vm/action"]');

    actionForms.forEach((form) => {
      form.addEventListener('submit', function () {
        const submitButton = form.querySelector('button[type="submit"]');
        const actionInput = form.querySelector('input[name="action"]');
        const action = actionInput ? actionInput.value : '';

        let loadingText = 'Processing...';
        switch (action) {
          case 'start':
            loadingText = 'Starting...';
            break;
          case 'stop':
            loadingText = 'Stopping...';
            break;
          case 'reboot':
            loadingText = 'Rebooting...';
            break;
          case 'shutdown':
            loadingText = 'Shutting down...';
            break;
          case 'reset':
            loadingText = 'Resetting...';
            break;
          default:
            break;
        }

        showLoadingOnButton(submitButton, loadingText);

        const allActionButtons = document.querySelectorAll('form[action="/vm/action"] button');
        allActionButtons.forEach((btn) => {
          if (btn !== submitButton) {
            btn.disabled = true;
            btn.style.opacity = '0.5';
          }
        });
      });
    });
  }

  function initNetworkToggles() {
    const toggleInputs = document.querySelectorAll('.network-toggle-input');
    toggleInputs.forEach(input => {
      input.addEventListener('change', function() {
        const index = this.dataset.networkIndex;
        const enabled = this.checked;
        const csrfToken = this.dataset.csrfToken;
        const vmid = this.dataset.vmid;
        const node = this.dataset.node;
        
        toggleNetworkCard(index, enabled, csrfToken, vmid, node);
      });
    });
  }

  document.addEventListener('DOMContentLoaded', function () {
    // Only start auto-refresh for running VMs
    const statusBadge = document.getElementById('vm-status-badge');
    if (statusBadge && statusBadge.textContent.includes('Running')) {
      startAutoRefresh();
    }

    // Pause refresh on user interaction
    document.addEventListener('click', pauseAutoRefresh);
    document.addEventListener('keypress', pauseAutoRefresh);
    
    // Show refresh indicator
    const refreshBtn = document.querySelector('a[href*="refresh=1"]');
    if (refreshBtn) {
      refreshBtn.addEventListener('click', function(e) {
        e.preventDefault();
        window.location.href = this.href;
      });
    }

    // Enhance VM action forms with loading indicators
    enhanceVMActionForms();
    
    // Initialize network toggle listeners
    initNetworkToggles();
  });

  // Expose functions used from inline HTML
  window.toggleNetworkCard = toggleNetworkCard;
  window.dismissCelebration = dismissCelebration;
  window.startVM = startVM;
  window.openConsole = openConsoleShortcut;
  window.editResources = editResources;
  window.showSuccess = showSuccess;
  window.showError = showError;
})();
