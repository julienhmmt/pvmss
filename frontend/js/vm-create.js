/**
 * VM Create Page JavaScript Module
 * Handles memory unit conversion and real-time display updates
 * Requires: vm-utils.js
 */

 (function() {
    'use strict';

    // Ensure VMUtils is available
    if (typeof VMUtils === 'undefined') {
        console.error('VMUtils not loaded. Ensure vm-utils.js is included before vm-create.js');
        return;
    }

    // DOM elements
    const memoryInput = document.getElementById('memory');
    const memoryUnit = document.getElementById('memory-unit');
    const memoryDisplay = document.getElementById('memory-display');
    
    // Early exit if elements not found
    if (!memoryInput || !memoryUnit || !memoryDisplay) {
        console.warn('VM Create: Required memory elements not found');
        return;
    }

    // Base limits in MB, derived from input attributes
    let minMB = 0;
    let maxMB = 0;

    /**
     * Update memory display with real-time conversion and adjust constraints
     */
    function updateMemoryDisplay() {
        const rawValue = memoryInput.value.trim();
        const numericValue = parseFloat(rawValue.replace(',', '.'));
        const value = Number.isFinite(numericValue) ? numericValue : 0;
        const unit = memoryUnit.value === 'GB' ? 'GB' : 'MB';
        let mb;
        let gb;

        if (unit === 'GB') {
            mb = Math.round(value * 1024);
            gb = value;
        } else {
            mb = Math.round(value);
            gb = (value / 1024).toFixed(1);
        }

        // Get selected label with fallback
        const selectedLabel = memoryDisplay.dataset.selectedLabel || 'Selected';

        // Update display text
        memoryDisplay.textContent = `${selectedLabel}: ${gb} GB (${mb} MB)`;

        // Adjust HTML constraints to match the current unit
        if (minMB > 0 && maxMB > 0 && maxMB >= minMB) {
            if (unit === 'GB') {
                const minGB = minMB / 1024;
                const maxGB = maxMB / 1024;
                memoryInput.min = minGB.toFixed(1);
                memoryInput.max = Math.round(maxGB).toString();
                memoryInput.step = '0.5';
            } else {
                memoryInput.min = String(minMB);
                memoryInput.max = String(maxMB);
                memoryInput.step = '256';
            }
        }
    }

    /**
     * Handle unit change with value conversion
     */
    function handleUnitChange() {
        const currentValue = parseFloat(memoryInput.value.replace(',', '.')) || 0;
        const lastUnit = memoryInput.dataset.lastUnit || 'MB';
        
        if (this.value === 'GB' && lastUnit === 'MB') {
            // Convert MB to GB, use decimal point
            const gbValue = currentValue / 1024;
            memoryInput.value = gbValue.toFixed(1);
        } else if (this.value === 'MB' && lastUnit === 'GB') {
            // Convert GB to MB, round to integer
            memoryInput.value = Math.round(currentValue * 1024);
        }
        
        memoryInput.dataset.lastUnit = this.value;
        updateMemoryDisplay();
    }

    /**
     * Initialize memory management
     */
    function init() {
        // Capture base limits from the rendered attributes (in MB)
        const minAttr = parseFloat(memoryInput.getAttribute('min') || '0');
        const maxAttr = parseFloat(memoryInput.getAttribute('max') || '0');
        if (Number.isFinite(minAttr) && Number.isFinite(maxAttr) && maxAttr >= minAttr && minAttr > 0) {
            minMB = minAttr;
            maxMB = maxAttr;
        }

        // Set initial unit
        memoryInput.dataset.lastUnit = memoryUnit.value || 'MB';
        
        // Add event listeners (no debounce to keep instant feedback like original inline script)
        memoryInput.addEventListener('input', updateMemoryDisplay);
        memoryInput.addEventListener('change', updateMemoryDisplay);
        memoryUnit.addEventListener('change', handleUnitChange);
        
        // Initial display update
        updateMemoryDisplay();
        
        console.log('VM Create: Memory management initialized');
    }

    // Initialize when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

    // Expose functions for external use (e.g., from inline scripts)
    window.VMCreate = {
        updateMemoryDisplay,
        handleUnitChange
    };

})();
