// VM Console functionality
document.addEventListener('DOMContentLoaded', function() {
    const consoleBtn = document.getElementById('toggle-console-btn');
    const consoleStatus = document.getElementById('console-status');
    const consoleContainer = document.getElementById('console-container');
    
    if (!consoleBtn || !consoleStatus) {
        console.log('Console elements not found on this page');
        return;
    }
    
    console.log('Console button found, setting up event listener');
    
    consoleBtn.addEventListener('click', async function() {
        // Get data from button attributes
        const vmid = this.dataset.vmid;
        const vmname = this.dataset.vmname;
        const node = this.dataset.node;
        
        console.log('Console button clicked with data:', { vmid, vmname, node });
        
        if (!vmid || !vmname) {
            console.error('Missing VM ID or name');
            consoleStatus.innerHTML = '<span class="has-text-danger">Error: Missing VM ID or name</span>';
            return;
        }
        
        // Show console container
        if (consoleContainer) {
            consoleContainer.classList.remove('is-hidden');
        }
        
        consoleBtn.classList.add('is-loading');
        consoleStatus.innerHTML = '<span class="has-text-info">🎫 Connecting to console...</span>';
        
        // Show loading overlay
        const loadingOverlay = document.getElementById('console-loading-overlay');
        if (loadingOverlay) {
            loadingOverlay.style.display = 'flex';
        }
        
        try {
            // Get host information - we need to get this from the backend since we don't know the Proxmox host from the frontend
            // Let's first try to get console info from the existing endpoint
            const formData = new FormData();
            formData.append('vmid', vmid);
            formData.append('vmname', vmname);
            if (node) {
                formData.append('node', node);
            }
            
            // Add CSRF token if available
            let csrfToken = document.querySelector('meta[name="csrf-token"]');
            if (csrfToken) {
                formData.append('csrf_token', csrfToken.getAttribute('content'));
            } else {
                // Fallback to hidden input
                csrfToken = document.querySelector('input[name="csrf_token"]');
                if (csrfToken) {
                    formData.append('csrf_token', csrfToken.value);
                }
            }
            
            console.log('🔑 Getting console info from backend...');
            
            // First get the console info (this will give us the host)
            const consoleInfoResponse = await fetch('/vm/console', {
                method: 'POST',
                body: formData,
                credentials: 'same-origin'
            });

            if (!consoleInfoResponse.ok) {
                const errorText = await consoleInfoResponse.text();
                throw new Error(errorText || `Failed to get console info: ${consoleInfoResponse.status}`);
            }
            
            const consoleInfo = await consoleInfoResponse.json();
            console.log('Console info received:', consoleInfo);
            console.log('🔍 Full JSON response:', JSON.stringify(consoleInfo, null, 2));

            if (!consoleInfo.success) {
                throw new Error(consoleInfo.message || 'Failed to get console info');
            }

            consoleStatus.innerHTML = '<span class="has-text-success">✅ Console access granted - loading console</span>';

            const url = consoleInfo.console_url || consoleInfo.url;
            if (!url) {
                throw new Error('Console URL missing from response');
            }
            
            console.log('🖥️ Loading console in iframe:', url);
            console.log('🔍 URL length:', url.length);
            console.log('🔍 URL contains host:', url.includes('host='));
            console.log('🔍 URL contains ticket:', url.includes('ticket='));
            
            // Load console in the existing iframe instead of opening a popup
            const iframe = document.getElementById('console-iframe');
            if (!iframe) {
                throw new Error('Console iframe not found');
            }
            
            // Set up iframe load event handlers
            iframe.onload = function() {
                console.log('✅ Console iframe loaded successfully');
                // Hide loading overlay after iframe loads
                if (loadingOverlay) {
                    setTimeout(() => {
                        loadingOverlay.style.display = 'none';
                    }, 1000); // Give it a moment to fully render
                }
                consoleStatus.innerHTML = '<span class="has-text-success">✅ Console ready</span>';
            };
            
            iframe.onerror = function() {
                console.error('❌ Console iframe failed to load');
                if (loadingOverlay) {
                    loadingOverlay.style.display = 'none';
                }
                throw new Error('Console iframe failed to load');
            };
            
            // Load the console URL in the iframe
            iframe.src = url;

            // Update button text
            const btnText = document.getElementById('console-btn-text');
            if (btnText) {
                btnText.textContent = 'Console Active';
            }

            // Hide status after success
            setTimeout(() => {
                if (consoleStatus) {
                    consoleStatus.style.display = 'none';
                }
            }, 5000);

        } catch (error) {
            console.error('Console error:', error);
            consoleStatus.innerHTML = `<span class="has-text-danger">❌ Error: ${error.message}</span>`;
            
            // Hide loading overlay on error
            if (loadingOverlay) {
                loadingOverlay.style.display = 'none';
            }

            // Show error message in iframe area
            const iframe = document.getElementById('console-iframe');
            if (iframe) {
                iframe.src = 'data:text/html,<html><body style="background:#2c3e50;color:white;font-family:Arial;display:flex;justify-content:center;align-items:center;height:100vh;margin:0;"><div style="text-align:center;"><h2>❌ Console Connection Failed</h2><p>' + error.message + '</p><p>Please check your Proxmox connection and try again.</p></div></body></html>';
            }
        } finally {
            consoleBtn.classList.remove('is-loading');
        }
    });
});
