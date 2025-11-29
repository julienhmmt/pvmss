/**
 * Search Page JavaScript Module
 * Handles AJAX-powered VM search functionality
 */

(function() {
  'use strict';

  // DOM elements
  const vmidSearch = document.getElementById('vmid-search');
  const nameSearch = document.getElementById('name-search');
  const tagsFilter = document.getElementById('tags-filter');
  const limitSelect = document.getElementById('limit-select');
  const searchBtn = document.getElementById('search-btn');
  const clearBtn = document.getElementById('clear-btn');
  const searchLoading = document.getElementById('search-loading');
  const searchError = document.getElementById('search-error');
  const errorMessage = document.getElementById('error-message');
  const errorClose = document.getElementById('error-close');
  const searchResults = document.getElementById('search-results');
  const resultsContainer = document.getElementById('results-container');

  // Early exit if elements not found
  if (!searchBtn || !resultsContainer) {
    return;
  }

  // Get translations from data attributes
  const translationsEl = document.getElementById('search-translations');
  const t = translationsEl ? translationsEl.dataset : {};

  const translations = {
    actions: t.actions || 'Actions',
    description: t.description || 'Description',
    details: t.details || 'Details',
    loadError: t.loadError || 'Failed to load results',
    name: t.name || 'Name',
    noDescription: t.noDescription || 'No description',
    noName: t.noName || 'No name',
    noResults: t.noResults || 'No results',
    noResultsMessage: t.noResultsMessage || 'Try adjusting your search criteria',
    noTags: t.noTags || 'No tags',
    node: t.node || 'Node',
    resultFound: t.resultFound || 'result found',
    resultsFound: t.resultsFound || 'results found',
    search: t.search || 'Search',
    searchError: t.searchError || 'Search error',
    searching: t.searching || 'Searching...',
    status: t.status || 'Status',
    tags: t.tags || 'Tags',
    vmid: t.vmid || 'VMID'
  };

  let searchTimeout = null;

  /**
   * Show loading state
   */
  function showLoading() {
    searchLoading.classList.remove('is-hidden');
    searchBtn.disabled = true;
    searchBtn.innerHTML = '<span class="icon"><i class="fas fa-spinner fa-spin"></i></span><span>' + translations.searching + '</span>';
  }

  /**
   * Hide loading state
   */
  function hideLoading() {
    searchLoading.classList.add('is-hidden');
    searchBtn.disabled = false;
    searchBtn.innerHTML = '<span class="icon"><i class="fas fa-search"></i></span><span>' + translations.search + '</span>';
  }

  /**
   * Show error message
   * @param {string} message - Error message to display
   */
  function showError(message) {
    errorMessage.textContent = message;
    searchError.classList.remove('is-hidden');
  }

  /**
   * Hide error message
   */
  function hideError() {
    searchError.classList.add('is-hidden');
  }

  /**
   * Show results section
   */
  function showResults() {
    searchResults.classList.remove('is-hidden');
  }

  /**
   * Hide results section
   */
  function hideResults() {
    searchResults.classList.add('is-hidden');
  }

  /**
   * Get CSS class for status badge
   * @param {string} status - VM status
   * @returns {string} CSS class
   */
  function getStatusBadgeClass(status) {
    switch (status) {
      case 'running': return 'is-success';
      case 'stopped': return 'is-danger';
      case 'paused':
      case 'suspended': return 'is-warning';
      default: return 'is-light';
    }
  }

  /**
   * Get icon class for status
   * @param {string} status - VM status
   * @returns {string} Icon class
   */
  function getStatusIcon(status) {
    switch (status) {
      case 'running': return 'fa-circle-play';
      case 'stopped': return 'fa-circle-stop';
      case 'paused':
      case 'suspended': return 'fa-circle-pause';
      default: return 'fa-question-circle';
    }
  }

  /**
   * Escape HTML to prevent XSS
   * @param {string} text - Text to escape
   * @returns {string} Escaped text
   */
  function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  /**
   * Render search results
   * @param {Array} results - Array of VM objects
   */
  function renderResults(results) {
    if (!results || results.length === 0) {
      resultsContainer.innerHTML = `
        <div class="notification is-warning">
          <div class="level">
            <div class="level-left">
              <div class="level-item">
                <div>
                  <strong>${escapeHtml(translations.noResults)}</strong>
                  <p class="mb-0">${escapeHtml(translations.noResultsMessage)}</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      `;
      return;
    }

    let html = `
      <div class="table-container">
        <table class="table is-fullwidth is-hoverable">
          <thead>
            <tr class="has-background-light">
              <th class="has-text-centered" width="80">${escapeHtml(translations.vmid)}</th>
              <th>${escapeHtml(translations.name)}</th>
              <th>${escapeHtml(translations.description)}</th>
              <th class="has-text-centered" width="100">${escapeHtml(translations.node)}</th>
              <th class="has-text-centered" width="80">${escapeHtml(translations.status)}</th>
              <th class="has-text-centered" width="120">${escapeHtml(translations.tags)}</th>
              <th class="has-text-centered" width="120">${escapeHtml(translations.actions)}</th>
            </tr>
          </thead>
          <tbody>
    `;

    results.forEach(vm => {
      const statusClass = getStatusBadgeClass(vm.status);
      const statusIcon = getStatusIcon(vm.status);
      const vmName = vm.name ? escapeHtml(vm.name) : `<em class="has-text-grey-light">${escapeHtml(translations.noName)}</em>`;
      const vmDesc = vm.description ? escapeHtml(vm.description) : `<em class="has-text-grey-light">${escapeHtml(translations.noDescription)}</em>`;

      let tagsHtml = `<em class="has-text-grey-light is-size-7">${escapeHtml(translations.noTags)}</em>`;
      if (vm.tags) {
        const tagList = vm.tags.split(/[,;]/)
          .map(tag => tag.trim())
          .filter(tag => tag && tag !== 'pvmss');
        if (tagList.length > 0) {
          tagsHtml = tagList.map(tag => `<span class="tag is-small is-info is-light">${escapeHtml(tag)}</span>`).join(' ');
        }
      }

      html += `
        <tr class="is-vcentered">
          <td class="has-text-centered">
            <span class="tag is-light is-medium has-text-weight-bold">${vm.vmid}</span>
          </td>
          <td>
            <div class="is-flex is-align-items-center">
              <span class="has-text-weight-semibold">${vmName}</span>
            </div>
          </td>
          <td>
            <span>${vmDesc}</span>
          </td>
          <td class="has-text-centered">
            <span class="tag is-medium">${escapeHtml(vm.node)}</span>
          </td>
          <td class="has-text-centered">
            <span class="tag ${statusClass} is-medium">
              <span class="icon is-small"><i class="fas ${statusIcon}"></i></span>
              <span>${escapeHtml(vm.status)}</span>
            </span>
          </td>
          <td class="has-text-centered">
            ${tagsHtml}
          </td>
          <td class="has-text-centered">
            <a href="/vm/details/${vm.vmid}" class="button is-small is-primary">
              <span class="icon"><i class="fas fa-eye"></i></span>
              <span>${escapeHtml(translations.details)}</span>
            </a>
          </td>
        </tr>
      `;
    });

    html += `
          </tbody>
        </table>
      </div>
    `;

    resultsContainer.innerHTML = html;
  }

  /**
   * Perform search API call
   */
  async function performSearch() {
    const params = new URLSearchParams({
      vmid: vmidSearch.value.trim(),
      name: nameSearch.value.trim(),
      tags: tagsFilter.value.trim(),
      limit: limitSelect.value
    });

    showLoading();
    hideError();

    try {
      const response = await fetch(`/api/search/vms?${params}`, {
        method: 'GET',
        headers: {
          'Accept': 'application/json',
          'Cache-Control': 'no-cache'
        }
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }

      const data = await response.json();

      if (data.success) {
        renderResults(data.results);
        showResults();
      } else {
        showError(data.error || translations.searchError);
        hideResults();
      }
    } catch (error) {
      console.error('Search error:', error);
      showError(translations.loadError);
      hideResults();
    } finally {
      hideLoading();
    }
  }

  /**
   * Debounced search
   */
  function debouncedSearch() {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(performSearch, 500);
  }

  /**
   * Clear search form
   */
  function clearSearch() {
    vmidSearch.value = '';
    nameSearch.value = '';
    tagsFilter.value = '';
    limitSelect.value = '50';
    hideResults();
    hideError();
  }

  // Event listeners
  searchBtn.addEventListener('click', performSearch);
  clearBtn.addEventListener('click', clearSearch);
  errorClose.addEventListener('click', hideError);

  // Auto-search on input changes (debounced)
  [vmidSearch, nameSearch, tagsFilter].forEach(input => {
    input.addEventListener('input', debouncedSearch);
  });

  limitSelect.addEventListener('change', debouncedSearch);

  // Search on Enter key
  [vmidSearch, nameSearch, tagsFilter].forEach(input => {
    input.addEventListener('keydown', function(e) {
      if (e.key === 'Enter') {
        e.preventDefault();
        performSearch();
      }
    });
  });

  // Initial search if we have any filters set
  if (vmidSearch.value || nameSearch.value || tagsFilter.value) {
    performSearch();
  }
})();
