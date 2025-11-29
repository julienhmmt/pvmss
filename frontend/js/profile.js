/**
 * Profile Page JavaScript Module
 * Handles VM list auto-refresh, stats updates, and table rendering
 */

(function () {
  'use strict';

  const card = document.getElementById('profile-vms-card');
  if (!card || card.dataset.hasError === '1') {
    return;
  }

  const refreshBtn = document.querySelector('[data-profile-refresh]');
  const translationsEl = document.getElementById('profile-translations');
  const labels = translationsEl ? translationsEl.dataset : {};

  const statusConfig = {
    running: { className: 'is-success', icon: 'fa-circle-play', label: labels.statusRunning || 'Running' },
    stopped: { className: 'is-danger', icon: 'fa-circle-stop', label: labels.statusStopped || 'Stopped' },
    paused: { className: 'is-warning', icon: 'fa-circle-pause', label: labels.statusPaused || 'Paused' },
    suspended: { className: 'is-warning', icon: 'fa-circle-pause', label: labels.statusPaused || 'Paused' },
    unknown: { className: 'is-warning', icon: 'fa-circle-question', label: labels.statusUnknown || 'Unknown' }
  };

  const statsColumns = document.getElementById('profile-stats-columns');
  const statsEmpty = document.getElementById('profile-stats-empty');
  const statsElements = {
    total: document.querySelector('[data-profile-total]'),
    running: document.querySelector('[data-profile-running]'),
    stopped: document.querySelector('[data-profile-stopped]')
  };

  const tableContainer = document.getElementById('profile-vm-table-container');
  const tableBody = document.getElementById('profile-vm-table-body');
  const emptyState = document.getElementById('profile-vm-empty-state');

  if (!tableBody) {
    return;
  }

  const parsedInterval = parseInt(card.dataset.refreshInterval || '45000', 10);
  const AUTO_REFRESH_MS = Number.isFinite(parsedInterval) ? parsedInterval : 45000;

  let refreshTimer = null;
  let isFetching = false;

  /**
   * Toggle hidden class on an element
   * @param {HTMLElement} element - Target element
   * @param {boolean} hide - Whether to hide the element
   */
  function toggleHidden(element, hide) {
    if (!element) return;
    element.classList.toggle('is-hidden', !!hide);
  }

  /**
   * Set loading state on refresh button
   * @param {boolean} isLoading - Loading state
   */
  function setLoading(isLoading) {
    if (!refreshBtn) return;
    refreshBtn.classList.toggle('is-loading', isLoading);
    refreshBtn.disabled = isLoading;
  }

  /**
   * Create a Font Awesome icon element
   * @param {string} iconClass - Icon class (without 'fas' prefix)
   * @returns {HTMLElement} Icon element
   */
  function createIcon(iconClass) {
    const icon = document.createElement('i');
    icon.className = `fas ${iconClass}`;
    return icon;
  }

  /**
   * Create a status badge element
   * @param {string} status - VM status (running, stopped, paused, etc.)
   * @returns {HTMLElement} Status badge element
   */
  function createStatusBadge(status) {
    const normalized = (status || '').toLowerCase();
    const config = statusConfig[normalized] || statusConfig.unknown;
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
   * Create an action button
   * @param {Object} options - Button options
   * @returns {HTMLElement} Button element
   */
  function createButton({ href, title, text, iconClass, className }) {
    const anchor = document.createElement('a');
    anchor.href = href;
    anchor.className = className;
    if (title) {
      anchor.title = title;
    }

    const iconSpan = document.createElement('span');
    iconSpan.className = 'icon';
    iconSpan.appendChild(createIcon(iconClass));

    const textSpan = document.createElement('span');
    textSpan.textContent = text;

    anchor.append(iconSpan, textSpan);
    return anchor;
  }

  /**
   * Create a placeholder element for empty values
   * @param {string} text - Placeholder text
   * @returns {HTMLElement} Placeholder element
   */
  function createPlaceholder(text) {
    const placeholder = document.createElement('em');
    placeholder.className = 'has-text-grey-light';
    placeholder.textContent = text;
    return placeholder;
  }

  /**
   * Create a table row for a VM
   * @param {Object} vm - VM data object
   * @returns {HTMLElement} Table row element
   */
  function createVMRow(vm) {
    const tr = document.createElement('tr');
    tr.className = 'is-size-6';

    // VMID cell
    const vmidCell = document.createElement('td');
    vmidCell.className = 'has-text-centered is-vcentered';
    const vmidTag = document.createElement('span');
    vmidTag.className = 'tag is-light is-medium has-text-weight-bold';
    vmidTag.textContent = String(vm.vmid ?? '');
    vmidCell.appendChild(vmidTag);
    tr.appendChild(vmidCell);

    // Name cell
    const nameCell = document.createElement('td');
    nameCell.className = 'is-vcentered';
    const nameWrapper = document.createElement('div');
    nameWrapper.className = 'is-flex is-align-items-center';
    const nameSpan = document.createElement('span');
    nameSpan.className = 'has-text-weight-semibold';
    const nameValue = typeof vm.name === 'string' ? vm.name.trim() : '';
    if (nameValue) {
      nameSpan.textContent = nameValue;
    } else {
      nameSpan.appendChild(createPlaceholder(labels.noName || ''));
    }
    nameWrapper.appendChild(nameSpan);
    nameCell.appendChild(nameWrapper);
    tr.appendChild(nameCell);

    // Description cell
    const descCell = document.createElement('td');
    descCell.className = 'is-vcentered';
    const descSpan = document.createElement('span');
    descSpan.className = 'is-size-7';
    const descValue = typeof vm.description === 'string' ? vm.description.trim() : '';
    if (descValue) {
      descSpan.textContent = descValue;
    } else {
      descSpan.appendChild(createPlaceholder(labels.noDescription || ''));
    }
    descCell.appendChild(descSpan);
    tr.appendChild(descCell);

    // Node cell
    const nodeCell = document.createElement('td');
    nodeCell.className = 'has-text-centered is-vcentered';
    const nodeTag = document.createElement('span');
    nodeTag.className = 'tag is-small';
    nodeTag.textContent = typeof vm.node === 'string' ? vm.node : '';
    nodeCell.appendChild(nodeTag);
    tr.appendChild(nodeCell);

    // Status cell
    const statusCell = document.createElement('td');
    statusCell.className = 'has-text-centered is-vcentered';
    statusCell.appendChild(createStatusBadge(vm.status));
    tr.appendChild(statusCell);

    // Actions cell
    const actionsCell = document.createElement('td');
    actionsCell.className = 'has-text-centered is-vcentered';
    const buttonGroup = document.createElement('div');
    buttonGroup.className = 'buttons is-centered mb-0';

    const viewDetailsText = labels.viewDetails || '';
    const deleteText = labels.delete || '';

    const detailsBtn = createButton({
      href: `/vm/details/${vm.vmid}`,
      title: viewDetailsText,
      text: viewDetailsText,
      iconClass: 'fa-eye',
      className: 'button is-small is-primary'
    });

    const deleteBtn = createButton({
      href: `/vm/delete/${vm.vmid}`,
      title: deleteText,
      text: deleteText,
      iconClass: 'fa-trash-alt',
      className: 'button is-small is-danger has-text-white'
    });

    buttonGroup.append(detailsBtn, deleteBtn);
    actionsCell.appendChild(buttonGroup);
    tr.appendChild(actionsCell);

    return tr;
  }

  /**
   * Update stats display
   * @param {Object} summary - Summary object with total, running, stopped counts
   */
  function updateStats(summary) {
    if (statsElements.total) {
      statsElements.total.textContent = String(summary.total ?? 0);
    }
    if (statsElements.running) {
      statsElements.running.textContent = String(summary.running ?? 0);
    }
    if (statsElements.stopped) {
      statsElements.stopped.textContent = String(summary.stopped ?? 0);
    }
  }

  /**
   * Update table with VM data
   * @param {Array} vms - Array of VM objects
   */
  function updateTable(vms) {
    tableBody.innerHTML = '';
    vms.forEach((vm) => {
      tableBody.appendChild(createVMRow(vm));
    });
  }

  /**
   * Update UI with new data
   * @param {Object} payload - API response payload
   */
  function updateUI(payload) {
    const vms = Array.isArray(payload.vms) ? payload.vms : [];
    const summary = payload.summary || {};
    const hasVMs = vms.length > 0;

    updateStats(summary);

    toggleHidden(statsColumns, !hasVMs);
    toggleHidden(statsEmpty, hasVMs);
    toggleHidden(tableContainer, !hasVMs);
    toggleHidden(emptyState, hasVMs);

    if (hasVMs) {
      updateTable(vms);
    } else {
      tableBody.innerHTML = '';
    }
  }

  /**
   * Fetch VMs from API and update UI
   * @param {Object} options - Options
   * @param {boolean} options.showLoading - Whether to show loading indicator
   */
  async function refreshVMs(options = {}) {
    const { showLoading = false } = options;
    if (isFetching) {
      return;
    }
    isFetching = true;
    if (showLoading) {
      setLoading(true);
    }
    try {
      const response = await fetch('/api/profile/vms', {
        headers: {
          'Accept': 'application/json'
        }
      });
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }
      const data = await response.json();
      if (data && data.status === 'success') {
        updateUI(data);
      }
    } catch (error) {
      console.warn('Failed to refresh profile VM list', error);
    } finally {
      if (showLoading) {
        setLoading(false);
      }
      isFetching = false;
    }
  }

  /**
   * Stop auto-refresh timer
   */
  function stopAutoRefresh() {
    if (refreshTimer) {
      clearInterval(refreshTimer);
      refreshTimer = null;
    }
  }

  /**
   * Start auto-refresh timer
   */
  function startAutoRefresh() {
    stopAutoRefresh();
    refreshTimer = setInterval(() => {
      if (document.hidden) {
        return;
      }
      refreshVMs();
    }, AUTO_REFRESH_MS);
  }

  // Event listeners
  if (refreshBtn) {
    refreshBtn.addEventListener('click', (event) => {
      event.preventDefault();
      refreshVMs({ showLoading: true });
    });
  }

  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) {
      refreshVMs();
    }
  });

  window.addEventListener('beforeunload', stopAutoRefresh);

  // Start auto-refresh
  startAutoRefresh();
})();
