// VM Console Management with noVNC
// This module handles the noVNC console connection for VM details page

import RFB from '/components/noVNC-1.6.0/core/rfb.js';

/**
 * Initialize console manager for a VM
 * @param {Object} config - Configuration object
 * @param {string} config.vmid - VM ID
 * @param {string} config.node - Proxmox node name
 * @param {string} config.csrfToken - CSRF token for API requests
 */
export function initConsoleManager(config) {
    const { vmid, node, csrfToken } = config;
    
    // DOM elements
    const consoleButton = document.getElementById('console-button');
    const consoleModal = document.getElementById('console-modal');
    const consoleClose = document.getElementById('console-close');
    const consoleDisconnect = document.getElementById('console-disconnect');
    const consoleStatus = document.getElementById('console-status-text');
    const consoleContainer = document.getElementById('console-container');
    const ctrlAltDelButton = document.getElementById('console-ctrl-alt-del');
    const scaleToggleButton = document.getElementById('console-scale-toggle');
    const fullscreenButton = document.getElementById('console-fullscreen');
    
    // noVNC connection state
    let rfb = null;
    let scaleViewport = true;
    
    /**
     * Update console status message and styling
     * @param {string} message - Status message to display
     * @param {string} type - Type: info, success, error, connecting
     */
    function updateStatus(message, type = 'info') {
        const statusDiv = document.getElementById('console-status');
        const icon = statusDiv.querySelector('.icon i');
        
        consoleStatus.textContent = message;
        
        // Update icon based on type
        const iconMap = {
            success: 'check-circle',
            error: 'exclamation-circle',
            connecting: 'circle-notch fa-spin',
            info: 'info-circle'
        };
        icon.className = `fas fa-${iconMap[type] || iconMap.info}`;
        
        // Update background color
        const bgMap = {
            success: '#d4edda',
            error: '#f8d7da',
            info: '#f5f5f5'
        };
        statusDiv.style.background = bgMap[type] || bgMap.info;
    }
    
    /**
     * Open console connection
     */
    async function openConsole() {
        try {
            updateStatus('Requesting console access...', 'connecting');
            
            // Get VNC ticket from backend
            const response = await fetch(`/api/vm/vnc-ticket?vmid=${vmid}&node=${node}`, {
                method: 'POST',
                headers: {
                    'X-CSRF-Token': csrfToken,
                    'Content-Type': 'application/json'
                },
                credentials: 'same-origin'
            });
            
            if (!response.ok) {
                const error = await response.json().catch(() => ({ error: response.statusText }));
                throw new Error(error.error || 'Failed to get console ticket');
            }
            
            const data = await response.json();
            
            if (!data.success) {
                throw new Error(data.error || 'Failed to get console ticket');
            }
            
            const { ticket, port } = data;
            
            // Build WebSocket URL
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = `${protocol}//${window.location.host}/vm/console/websocket?vmid=${vmid}&node=${node}&port=${port}&vncticket=${encodeURIComponent(ticket)}`;
            
            updateStatus('Connecting to console...', 'connecting');
            
            // Create noVNC connection
            rfb = new RFB(consoleContainer, wsUrl, {
                credentials: { 
                    username: '',
                    password: ticket
                }
            });
            
            // Configure viewport settings
            rfb.scaleViewport = scaleViewport;
            rfb.resizeSession = false;
            rfb.clipViewport = false;
            
            // Set up event handlers
            rfb.addEventListener('connect', handleConnect);
            rfb.addEventListener('disconnect', handleDisconnect);
            rfb.addEventListener('securityfailure', handleSecurityFailure);
            
            // Enable console controls
            ctrlAltDelButton.disabled = false;
            scaleToggleButton.disabled = false;
            fullscreenButton.disabled = false;
            
            updateScaleButton();
            
            console.log('Console connection initiated');
            
        } catch (error) {
            console.error('Console error:', error);
            updateStatus(error.message, 'error');
            
            // Show error with retry option
            setTimeout(() => {
                if (confirm(`Failed to open console: ${error.message}\n\nWould you like to try again?`)) {
                    closeConsole();
                    setTimeout(openConsole, 500);
                } else {
                    closeConsole();
                }
            }, 500);
        }
    }
    
    /**
     * Handle successful connection
     */
    function handleConnect() {
        console.log('Console connected');
        updateStatus('Connected', 'success');
        
        // Focus the noVNC canvas
        const canvas = consoleContainer.querySelector('canvas');
        if (canvas) {
            canvas.focus();
        }
    }
    
    /**
     * Handle disconnection
     * @param {Event} e - Disconnect event
     */
    function handleDisconnect(e) {
        console.log('Console disconnected:', e.detail);
        
        if (e.detail.clean) {
            updateStatus('Disconnected', 'info');
        } else {
            updateStatus(`Connection lost: ${e.detail.reason || 'Unknown error'}`, 'error');
        }
        
        // Disable controls
        ctrlAltDelButton.disabled = true;
        scaleToggleButton.disabled = true;
        fullscreenButton.disabled = true;
        
        // Auto-close modal after disconnect error
        if (!e.detail.clean) {
            setTimeout(() => {
                if (consoleModal.classList.contains('is-active')) {
                    closeConsole();
                }
            }, 3000);
        }
    }
    
    /**
     * Handle security/authentication failure
     * @param {Event} e - Security failure event
     */
    function handleSecurityFailure(e) {
        console.error('Security failure:', e.detail);
        updateStatus(`Authentication failed: ${e.detail.reason || 'Security error'}`, 'error');
        
        setTimeout(() => {
            closeConsole();
        }, 3000);
    }
    
    /**
     * Close console and cleanup
     */
    function closeConsole() {
        // Disconnect noVNC
        if (rfb) {
            try {
                rfb.disconnect();
            } catch (e) {
                console.error('Error disconnecting:', e);
            }
            rfb = null;
        }
        
        // Clear container
        consoleContainer.innerHTML = '';
        
        // Reset status
        updateStatus('Disconnected', 'info');
        
        // Disable controls
        ctrlAltDelButton.disabled = true;
        scaleToggleButton.disabled = true;
        fullscreenButton.disabled = true;
        
        // Close modal
        consoleModal.classList.remove('is-active');
    }
    
    /**
     * Toggle viewport scaling
     */
    function toggleScale() {
        if (rfb) {
            scaleViewport = !scaleViewport;
            rfb.scaleViewport = scaleViewport;
            updateScaleButton();
            console.log('Viewport scaling:', scaleViewport ? 'enabled' : 'disabled');
        }
    }
    
    /**
     * Update scale button appearance
     */
    function updateScaleButton() {
        if (scaleViewport) {
            scaleToggleButton.classList.add('is-info');
            scaleToggleButton.classList.remove('is-light');
            const span = scaleToggleButton.querySelector('span:not(.icon)');
            if (span) span.textContent = 'Scale: On';
            scaleToggleButton.title = 'Scaling enabled - Click to disable and show original size';
        } else {
            scaleToggleButton.classList.remove('is-info');
            scaleToggleButton.classList.add('is-light');
            const span = scaleToggleButton.querySelector('span:not(.icon)');
            if (span) span.textContent = 'Scale: Off';
            scaleToggleButton.title = 'Scaling disabled - Click to enable responsive scaling';
        }
    }
    
    /**
     * Send Ctrl+Alt+Del to remote machine
     */
    function sendCtrlAltDel() {
        if (rfb) {
            rfb.sendCtrlAltDel();
            console.log('Sent Ctrl+Alt+Del');
        }
    }
    
    /**
     * Toggle fullscreen mode
     */
    function toggleFullscreen() {
        const modalCard = consoleModal.querySelector('.modal-card');
        
        if (!document.fullscreenElement) {
            modalCard.requestFullscreen().catch(err => {
                console.error('Error attempting to enable fullscreen:', err);
            });
        } else {
            document.exitFullscreen();
        }
    }
    
    // Event listeners
    if (consoleButton) {
        consoleButton.addEventListener('click', function() {
            consoleModal.classList.add('is-active');
            // Small delay to ensure modal is visible before connecting
            setTimeout(openConsole, 100);
        });
    }
    
    if (consoleClose) {
        consoleClose.addEventListener('click', closeConsole);
    }
    
    if (consoleDisconnect) {
        consoleDisconnect.addEventListener('click', closeConsole);
    }
    
    const modalBackground = consoleModal?.querySelector('.modal-background');
    if (modalBackground) {
        modalBackground.addEventListener('click', function() {
            if (confirm('Are you sure you want to close the console?')) {
                closeConsole();
            }
        });
    }
    
    if (ctrlAltDelButton) {
        ctrlAltDelButton.addEventListener('click', sendCtrlAltDel);
    }
    
    if (scaleToggleButton) {
        scaleToggleButton.addEventListener('click', toggleScale);
    }
    
    if (fullscreenButton) {
        fullscreenButton.addEventListener('click', toggleFullscreen);
    }
    
    // Cleanup on page unload
    window.addEventListener('beforeunload', function() {
        if (rfb) {
            rfb.disconnect();
        }
    });
    
    return {
        open: openConsole,
        close: closeConsole,
        isConnected: () => rfb !== null
    };
}
