/**
 * Profile Page JavaScript Module
 * Handles VM list auto-refresh, stats updates, and table rendering
 * Requires: vm-utils.js
 */

(function () {
  'use strict';

  // Ensure VMUtils is available
  if (typeof VMUtils === 'undefined') {
    console.error('VMUtils not loaded. Ensure vm-utils.js is included before profile.js');
    return;
  }

  const card = document.getElementById('profile-vms-card');
  if (!card || card.dataset.hasError === '1') {
    return;
  }

  const refreshBtn = document.querySelector('[data-profile-refresh]');
  const translationsEl = document.getElementById('profile-translations');
  const labels = translationsEl ? translationsEl.dataset : {};

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
   * Set loading state on refresh button
   * @param {boolean} isLoading - Loading state
   */
  function setLoading(isLoading) {
    if (!refreshBtn) return;
    refreshBtn.classList.toggle('is-loading', isLoading);
    refreshBtn.disabled = isLoading;
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
    iconSpan.appendChild(VMUtils.createIcon(iconClass));

    const textSpan = document.createElement('span');
    textSpan.textContent = text;

    anchor.append(iconSpan, textSpan);
    return anchor;
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
      nameSpan.appendChild(VMUtils.createPlaceholder(labels.noName || ''));
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
      descSpan.appendChild(VMUtils.createPlaceholder(labels.noDescription || ''));
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
    statusCell.appendChild(VMUtils.createStatusBadge(vm.status, labels));
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

    VMUtils.toggleHidden(statsColumns, !hasVMs);
    VMUtils.toggleHidden(statsEmpty, hasVMs);
    VMUtils.toggleHidden(tableContainer, !hasVMs);
    VMUtils.toggleHidden(emptyState, hasVMs);

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
      const result = await VMUtils.fetchJson('/api/profile/vms');
      if (!result.success) {
        throw new Error(result.error);
      }
      const data = result.data;
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
