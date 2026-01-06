/**
 * VM Console Initialization Module
 * Loads and initializes the noVNC console manager
 */

import { initConsoleManager } from '/js/vm-console.js';

/**
 * Initialize noVNC console manager once the VM details globals and DOM are available.
 * @returns {void}
 */
function initVMConsole() {
    /** @type {string|number|undefined} */
    const vmid = window.vmDetailsVMID;
    /** @type {string|undefined} */
    const node = window.vmDetailsNode;
    if (!vmid || !node) {
        console.warn('VM details not available for console initialization');
        return;
    }
    /** @type {string|undefined} */
    const status = window.vmDetailsStatus;
    const configEl = document.getElementById('vm-details-config');
    const cfg = configEl ? configEl.dataset : {};
    const consoleMustBeRunning = cfg.consoleMustBeRunning || 'VM must be running to open console. Please start the VM first.';
    const consoleButton = document.getElementById('console-button');
    if (consoleButton && status && status !== 'running') {
        consoleButton.addEventListener('click', function(e) {
            e.preventDefault();
            e.stopImmediatePropagation();
            if (typeof window.showError === 'function') {
                window.showError(consoleMustBeRunning);
                return;
            }
            alert(consoleMustBeRunning);
        }, true);
    }
    const csrfMeta = document.querySelector('meta[name="csrf-token"]');
    const csrfToken = csrfMeta ? csrfMeta.getAttribute('content') : '';
    const titleMatch = document.title.match(/^(.+?)\s*\(/);
    const vmName = titleMatch ? titleMatch[1] : `VM ${vmid}`;
    console.log('Initializing console manager for VM:', { vmid, node, vmName });
    try {
        initConsoleManager({
            vmid: String(vmid),
            node: String(node),
            vmName: String(vmName),
            csrfToken: String(csrfToken)
        });
        console.log('Console manager initialized successfully');
    } catch (error) {
        console.error('Failed to initialize console manager:', error);
    }
}

// Initialize console when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initVMConsole);
} else {
    initVMConsole();
}
