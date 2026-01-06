/**
 * VM Console Initialization Module
 * Loads and initializes the noVNC console manager
 */

import { initConsoleManager } from '/js/vm-console.js';

// Wait for DOM and global variables to be ready
document.addEventListener('DOMContentLoaded', function() {
    // Get VM details from global variables set by the template
    const vmid = window.vmDetailsVMID;
    const node = window.vmDetailsNode;
    
    if (!vmid || !node) {
        console.warn('VM details not available for console initialization');
        return;
    }
    
    // Get CSRF token from meta tag
    const csrfMeta = document.querySelector('meta[name="csrf-token"]');
    const csrfToken = csrfMeta ? csrfMeta.getAttribute('content') : '';
    
    // Get VM name from page title or status badge
    const titleMatch = document.title.match(/^(.+?)\s*\(/);
    const vmName = titleMatch ? titleMatch[1] : `VM ${vmid}`;
    
    console.log('Initializing console manager for VM:', { vmid, node, vmName });
    
    // Initialize console manager
    try {
        initConsoleManager({
            vmid: String(vmid),
            node: node,
            vmName: vmName,
            csrfToken: csrfToken
        });
        console.log('Console manager initialized successfully');
    } catch (error) {
        console.error('Failed to initialize console manager:', error);
    }
});
