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
                throw new Error(`Failed to get console info: ${consoleInfoResponse.status}`);
            }
            
            const consoleInfo = await consoleInfoResponse.json();
            console.log('Console info received:', consoleInfo);
            
            if (!consoleInfo.success) {
                throw new Error(consoleInfo.message || 'Failed to get console info');
            }
            
            // Now use individual user authentication with the correct host
            let authData = {
                host: consoleInfo.host,
                node: node,
                vmid: vmid
            };

            // First try without credentials (in case user session has stored credentials)
            let response = await fetch('/vm/console-auth', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                credentials: 'same-origin',
                body: JSON.stringify(authData)
            });

            let data;
            try {
                data = await response.json();
            } catch (jsonError) {
                // If JSON parsing fails, get the raw text to see what we got
                const rawText = await response.text();
                console.error('Failed to parse JSON response:', rawText);
                throw new Error(`Server returned invalid JSON: ${rawText}`);
            }
            
            // If we need user authentication, prompt for credentials
            if (!data.success && data.message === 'NEED_USER_AUTH') {
                console.log('🔐 User authentication required, prompting for credentials');
                
                // Update status to show we need authentication
                consoleStatus.innerHTML = '<span class="has-text-warning">🔐 Please enter your Proxmox credentials</span>';
                
                // Show authentication prompt
                const username = prompt('Enter your Proxmox username (without @pve):');
                const password = prompt('Enter your Proxmox password:');
                
                if (!username || !password) {
                    throw new Error('Authentication cancelled by user');
                }
                
                console.log('🔑 Retrying with user credentials for:', username);
                
                // Retry with credentials
                authData.username = username;
                authData.password = password;
                
                response = await fetch('/vm/console-auth', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    credentials: 'same-origin',
                    body: JSON.stringify(authData)
                });
                
                // Parse the response properly
                if (!response.ok) {
                    let errorText;
                    try {
                        const errorData = await response.json();
                        errorText = errorData.message || `HTTP ${response.status}`;
                    } catch (e) {
                        errorText = await response.text();
                    }
                    console.error('Console auth request failed:', errorText);
                    throw new Error(`Authentication failed: ${response.status} - ${errorText}`);
                }
                
                try {
                    data = await response.json();
                } catch (jsonError) {
                    const rawText = await response.text();
                    console.error('Failed to parse retry JSON response:', rawText);
                    throw new Error(`Server returned invalid JSON on retry: ${rawText}`);
                }
                console.log('🔑 Authentication response:', data);
            }
            
            console.log('Console response status:', response.status);
            
            if (!response.ok) {
                const errorText = await response.text();
                console.error('Console request failed:', errorText);
                throw new Error(errorText || `Server returned ${response.status}`);
            }
            
            console.log('Console data received:', data);
            
            if (!data.success) {
                throw new Error(data.message || 'Failed to get console access');
            }
            
            // Update status
            consoleStatus.innerHTML = '<span class="has-text-success">✅ Console connected successfully</span>';
            
            // Load the console in the iframe
            const iframe = document.getElementById('console-iframe');
            if (iframe) {
                console.log('🖥️ Loading console in iframe:', data.console_url);
                iframe.src = data.console_url;
                
                // Add iframe event listeners for debugging
                iframe.onload = function() {
                    console.log('✅ Console iframe loaded successfully');
                    // Hide loading overlay after iframe loads
                    setTimeout(() => {
                        if (loadingOverlay) {
                            loadingOverlay.style.display = 'none';
                        }
                    }, 2000);
                };
                
                iframe.onerror = function() {
                    console.error('❌ Console iframe failed to load');
                    if (loadingOverlay) {
                        loadingOverlay.style.display = 'none';
                    }
                };
                
                // Also hide loading overlay after a timeout as fallback
                setTimeout(() => {
                    if (loadingOverlay) {
                        loadingOverlay.style.display = 'none';
                    }
                    console.log('🕐 Loading overlay hidden after timeout');
                }, 5000);
            } else {
                console.error('❌ Console iframe not found');
            }
            
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
