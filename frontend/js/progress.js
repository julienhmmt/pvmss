// Global Progress Bar System
(function() {
    'use strict';

    class ProgressManager {
        constructor() {
            this.container = null;
            this.progresses = new Map();
            this.init();
        }

        init() {
            // Create progress container
            this.container = document.createElement('div');
            this.container.id = 'global-progress';
            this.container.className = 'global-progress';
            this.container.setAttribute('aria-live', 'polite');
            this.container.setAttribute('aria-label', 'Progress indicators');
            document.body.appendChild(this.container);

            // Listen for custom events
            document.addEventListener('show-progress', (e) => {
                this.show(e.detail);
            });

            document.addEventListener('update-progress', (e) => {
                this.update(e.detail.id, e.detail);
            });

            document.addEventListener('hide-progress', (e) => {
                this.hide(e.detail.id);
            });

            document.addEventListener('clear-progress', () => {
                this.clear();
            });
        }

        show(options) {
            const id = options.id || 'progress-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);

            // Create progress element
            const progress = this.createProgressElement(id, options);
            this.container.appendChild(progress);

            // Store progress data
            this.progresses.set(id, {
                element: progress,
                options: options,
                startTime: Date.now()
            });

            // Announce to screen readers
            this.announceProgress(options);

            return id;
        }

        createProgressElement(id, options) {
            const progress = document.createElement('div');
            progress.id = id;
            progress.className = `progress-notification is-${options.type || 'info'} ${options.showDetails !== false ? 'has-details' : ''}`;
            progress.setAttribute('role', 'status');
            progress.setAttribute('aria-live', 'polite');

            const progressBar = document.createElement('div');
            progressBar.className = 'progress-bar';

            const progressFill = document.createElement('div');
            progressFill.className = 'progress-fill';
            progressFill.style.width = '0%';

            progressBar.appendChild(progressFill);

            let html = `
                <div class="progress-content">
                    <div class="progress-header">
                        <div class="progress-icon">
                            <span class="icon">
                                <i class="${options.icon || 'fas fa-spinner fa-spin'}"></i>
                            </span>
                        </div>
                        <div class="progress-text">
                            <strong>${this.escapeHtml(options.title || 'Processing...')}</strong>
                            ${options.message ? `<p class="mb-0">${this.escapeHtml(options.message)}</p>` : ''}
                        </div>
                        <div class="progress-close">
                            ${options.showClose !== false ? '<button class="delete" aria-label="Cancel"></button>' : ''}
                        </div>
                    </div>
                    <div class="progress-bar-container">
            `;

            html += progressBar.outerHTML;
            html += `
                    </div>
                    ${options.showDetails !== false ? `
                    <div class="progress-details">
                        <span class="progress-percent">0%</span>
                        <span class="progress-time">--</span>
                    </div>
                    ` : ''}
                </div>
            `;

            progress.innerHTML = html;

            // Add event listeners
            this.attachEventListeners(progress, id, options);

            return progress;
        }

        attachEventListeners(progress, id, options) {
            // Close button
            const closeBtn = progress.querySelector('.delete');
            if (closeBtn) {
                closeBtn.addEventListener('click', () => {
                    if (options.onCancel) {
                        options.onCancel();
                    }
                    this.hide(id);
                });
            }
        }

        update(id, updates) {
            const progressData = this.progresses.get(id);
            if (!progressData) return;

            const { element, options, startTime } = progressData;

            // Update progress percentage
            if (updates.progress !== undefined) {
                const progressFill = element.querySelector('.progress-fill');
                const progressPercent = element.querySelector('.progress-percent');

                if (progressFill) {
                    progressFill.style.width = Math.min(100, Math.max(0, updates.progress)) + '%';
                }

                if (progressPercent) {
                    progressPercent.textContent = Math.round(updates.progress) + '%';
                }

                // Update icon based on progress
                if (updates.progress >= 100) {
                    const icon = element.querySelector('.progress-icon i');
                    if (icon) {
                        icon.className = 'fas fa-check-circle';
                        icon.style.color = '#48c774';
                    }
                }
            }

            // Update message
            if (updates.message) {
                const messageEl = element.querySelector('.progress-text p');
                if (messageEl) {
                    messageEl.textContent = this.escapeHtml(updates.message);
                }
            }

            // Update time
            const timeEl = element.querySelector('.progress-time');
            if (timeEl && startTime) {
                const elapsed = Date.now() - startTime;
                const seconds = Math.floor(elapsed / 1000);
                const minutes = Math.floor(seconds / 60);
                const remainingSeconds = seconds % 60;

                if (minutes > 0) {
                    timeEl.textContent = `${minutes}m ${remainingSeconds}s`;
                } else {
                    timeEl.textContent = `${remainingSeconds}s`;
                }
            }

            // Check if complete
            if (updates.progress >= 100) {
                setTimeout(() => {
                    this.hide(id);
                }, 2000); // Auto-hide after 2 seconds when complete
            }
        }

        hide(id) {
            const progressData = this.progresses.get(id);
            if (!progressData) return;

            const { element } = progressData;

            // Add fade out animation
            element.classList.add('is-hiding');

            // Remove after animation
            setTimeout(() => {
                if (element.parentNode) {
                    element.parentNode.removeChild(element);
                }
                this.progresses.delete(id);
            }, 300);
        }

        clear() {
            Array.from(this.progresses.keys()).forEach(id => {
                this.hide(id);
            });
        }

        announceProgress(options) {
            // Create a temporary element for screen reader announcement
            const announcement = document.createElement('div');
            announcement.setAttribute('aria-live', 'assertive');
            announcement.setAttribute('aria-atomic', 'true');
            announcement.style.position = 'absolute';
            announcement.style.left = '-10000px';
            announcement.style.width = '1px';
            announcement.style.height = '1px';
            announcement.style.overflow = 'hidden';

            announcement.textContent = `Progress started: ${options.title || 'Processing...'}`;
            document.body.appendChild(announcement);

            // Remove after announcement
            setTimeout(() => {
                document.body.removeChild(announcement);
            }, 1000);
        }

        escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }
    }

    // Global functions for easy access
    window.ProgressManager = new ProgressManager();

    window.showProgress = function(options) {
        return window.ProgressManager.show(options);
    };

    window.updateProgress = function(id, updates) {
        window.ProgressManager.update(id, updates);
    };

    window.hideProgress = function(id) {
        window.ProgressManager.hide(id);
    };

    window.clearProgress = function() {
        window.ProgressManager.clear();
    };

    // Initialize when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            // Ready
        });
    }
})();
