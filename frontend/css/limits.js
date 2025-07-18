/**
 * Resource Limits Management
 * This script provides minimal UI interaction for setting resource limits for Proxmox nodes and VMs.
 * Most logic is handled on the backend.
 */

document.addEventListener('DOMContentLoaded', function() {
    // Translations for UI messages
    const i18n = {
        saving: document.getElementById('i18n-saving')?.textContent || 'Saving...',
        saved: document.getElementById('i18n-saved')?.textContent || 'Saved',
        error: document.getElementById('i18n-error')?.textContent || 'Error',
        validation: document.getElementById('i18n-validation')?.textContent || 'Validation error'
    };

    // Show status message in the UI
    function showStatus(entityId, status) {
        const statusElement = document.querySelector(`.status-indicator[data-entity="${entityId}"]`);
        if (!statusElement) return;
        
        statusElement.textContent = status.text;
        statusElement.className = `status-indicator is-size-7 ${status.type || ''}`;
        
        if (status.timeout) {
            setTimeout(() => {
                if (statusElement.textContent === status.text) {
                    statusElement.textContent = '';
                    statusElement.className = 'status-indicator is-size-7';
                }
            }, status.timeout);
        }
    }

    // Restore default limits
    async function restoreDefaults(entityId) {
        if (!confirm('Are you sure you want to reset to default limits?')) return;

        try {
            const response = await fetch('/api/limits', {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'X-Requested-With': 'XMLHttpRequest'
                },
                body: JSON.stringify({ entityId: entityId })
            });

            if (!response.ok) {
                throw new Error('Failed to reset limits');
            }

            const result = await response.json();
            if (result.success) {
                showStatus(entityId, {
                    text: 'Defaults restored',
                    type: 'has-text-success',
                    timeout: 2000
                });
                // Reload the page to reflect changes
                setTimeout(() => location.reload(), 2000);
            } else {
                throw new Error(result.message || 'Failed to reset limits');
            }
        } catch (error) {
            console.error('Error resetting limits:', error);
            showStatus(entityId, {
                text: error.message || i18n.error,
                type: 'has-text-danger',
                timeout: 3000
            });
        }
    }

    // Save limits to backend
    async function saveLimits(entityId) {
        showStatus(entityId, {
            text: i18n.saving,
            type: 'has-text-info'
        });

        const form = document.querySelector(`.limits-form[data-entity-id="${entityId}"]`);
        if (!form) {
            showStatus(entityId, {
                text: i18n.error,
                type: 'has-text-danger',
                timeout: 3000
            });
            return;
        }

        const formData = new FormData(form);
        const payload = { entityId: entityId };
        for (const [key, value] of formData.entries()) {
            const [resource, type] = key.split('-');
            if (!payload[resource]) payload[resource] = {};
            payload[resource][type] = parseInt(value) || 0;
        }

        try {
            const response = await fetch('/api/limits', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-Requested-With': 'XMLHttpRequest'
                },
                body: JSON.stringify(payload)
            });

            const result = await response.json();
            if (!response.ok || !result.success) {
                throw new Error(result.message || 'Failed to save limits');
            }
            
            showStatus(entityId, {
                text: i18n.saved,
                type: 'has-text-success',
                timeout: 2000
            });
        } catch (error) {
            console.error('Error saving limits:', error);
            showStatus(entityId, {
                text: error.message || i18n.error,
                type: 'has-text-danger',
                timeout: 3000
            });
        }
    }

    // Add event listeners to all reset buttons
    document.querySelectorAll('.reset-defaults-btn').forEach(button => {
        const entityId = button.getAttribute('data-entity-id');
        if (!entityId) return;

        button.addEventListener('click', (e) => {
            e.preventDefault();
            restoreDefaults(entityId);
        });
    });

    // Add event listeners to all save buttons
    document.querySelectorAll('.save-limits-btn').forEach(button => {
        const entityId = button.getAttribute('data-entity-id');
        if (!entityId) return;

        button.addEventListener('click', (e) => {
            e.preventDefault();
            saveLimits(entityId);
        });
    });
});
