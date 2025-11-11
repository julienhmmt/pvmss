// Global Notification System
(function() {
    'use strict';

    class NotificationManager {
        constructor() {
            this.container = null;
            this.notifications = new Map();
            this.init();
        }

        init() {
            // Create notification container
            this.container = document.createElement('div');
            this.container.id = 'global-notifications';
            this.container.className = 'global-notifications';
            this.container.setAttribute('aria-live', 'polite');
            this.container.setAttribute('aria-label', 'Notifications');
            document.body.appendChild(this.container);

            // Listen for custom events
            document.addEventListener('show-notification', (e) => {
                this.show(e.detail);
            });

            document.addEventListener('hide-notification', (e) => {
                this.hide(e.detail.id);
            });

            document.addEventListener('clear-notifications', () => {
                this.clear();
            });
        }

        show(options) {
            const id = options.id || 'notification-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);

            // Create notification element
            const notification = this.createNotificationElement(id, options);
            this.container.appendChild(notification);

            // Store notification data
            this.notifications.set(id, {
                element: notification,
                options: options,
                timeout: null
            });

            // Auto-dismiss if specified
            if (options.autoDismiss !== false && options.duration) {
                this.notifications.get(id).timeout = setTimeout(() => {
                    this.hide(id);
                }, options.duration);
            }

            // Announce to screen readers
            this.announceNotification(options);

            return id;
        }

        createNotificationElement(id, options) {
            const notification = document.createElement('div');
            notification.id = id;
            notification.className = `notification is-${options.type || 'info'} ${options.dismissible !== false ? 'is-dismissible' : ''} global-notification`;
            notification.setAttribute('role', 'alert');
            notification.setAttribute('aria-live', options.urgent ? 'assertive' : 'polite');

            let html = '<div class="level">';

            // Left side - icon and content
            html += '<div class="level-left">';

            // Icon
            if (options.icon) {
                html += `<div class="level-item">
                    <span class="icon">
                        <i class="${options.icon}"></i>
                    </span>
                </div>`;
            }

            // Content
            html += '<div class="level-item"><div>';

            if (options.title) {
                html += `<strong>${this.escapeHtml(options.title)}</strong>`;
            }

            if (options.message) {
                html += `<p class="mb-0">${this.escapeHtml(options.message)}</p>`;
            }

            html += '</div></div></div>';

            // Right side - actions and dismiss
            html += '<div class="level-right">';

            // Actions
            if (options.actions && options.actions.length > 0) {
                html += '<div class="level-item"><div class="buttons are-small">';
                options.actions.forEach(action => {
                    const actionClass = action.class || 'is-primary';
                    const actionIcon = action.icon || 'fas fa-arrow-right';

                    if (action.url) {
                        html += `<a href="${action.url}" class="button ${actionClass} is-small">
                            <span class="icon is-small"><i class="${actionIcon}"></i></span>
                            <span>${this.escapeHtml(action.label)}</span>
                        </a>`;
                    } else if (action.callback) {
                        const callbackId = 'callback-' + Math.random().toString(36).substr(2, 9);
                        html += `<button class="button ${actionClass} is-small" data-callback="${callbackId}">
                            <span class="icon is-small"><i class="${actionIcon}"></i></span>
                            <span>${this.escapeHtml(action.label)}</span>
                        </button>`;
                        // Store callback
                        notification.dataset[callbackId] = action.callback;
                    }
                });
                html += '</div></div>';
            }

            // Dismiss button
            if (options.dismissible !== false) {
                html += `<div class="level-item">
                    <button class="delete" aria-label="Close notification"></button>
                </div>`;
            }

            html += '</div></div>';

            notification.innerHTML = html;

            // Add event listeners
            this.attachEventListeners(notification, id, options);

            return notification;
        }

        attachEventListeners(notification, id, options) {
            // Dismiss button
            const dismissBtn = notification.querySelector('.delete');
            if (dismissBtn) {
                dismissBtn.addEventListener('click', () => {
                    this.hide(id);
                });
            }

            // Action callbacks
            if (options.actions) {
                options.actions.forEach(action => {
                    if (action.callback) {
                        const callbackId = 'callback-' + Math.random().toString(36).substr(2, 9);
                        const btn = notification.querySelector(`[data-callback="${callbackId}"]`);
                        if (btn) {
                            btn.addEventListener('click', () => {
                                action.callback();
                                if (action.autoHide !== false) {
                                    this.hide(id);
                                }
                            });
                        }
                    }
                });
            }

            // Progress bar animation for auto-dismiss
            if (options.duration) {
                const progressBar = document.createElement('div');
                progressBar.className = 'notification-progress';
                progressBar.style.animationDuration = `${options.duration}ms`;
                notification.appendChild(progressBar);
            }
        }

        hide(id) {
            const notificationData = this.notifications.get(id);
            if (!notificationData) return;

            const { element, timeout } = notificationData;

            // Clear timeout
            if (timeout) {
                clearTimeout(timeout);
            }

            // Add fade out animation
            element.classList.add('is-hiding');

            // Remove after animation
            setTimeout(() => {
                if (element.parentNode) {
                    element.parentNode.removeChild(element);
                }
                this.notifications.delete(id);
            }, 300);
        }

        clear() {
            Array.from(this.notifications.keys()).forEach(id => {
                this.hide(id);
            });
        }

        announceNotification(options) {
            // Create a temporary element for screen reader announcement
            const announcement = document.createElement('div');
            announcement.setAttribute('aria-live', 'assertive');
            announcement.setAttribute('aria-atomic', 'true');
            announcement.style.position = 'absolute';
            announcement.style.left = '-10000px';
            announcement.style.width = '1px';
            announcement.style.height = '1px';
            announcement.style.overflow = 'hidden';

            announcement.textContent = `${options.title || 'Notification'}: ${options.message || ''}`;
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
    window.NotificationManager = new NotificationManager();

    window.showNotification = function(options) {
        return window.NotificationManager.show(options);
    };

    window.hideNotification = function(id) {
        window.NotificationManager.hide(id);
    };

    window.clearNotifications = function() {
        window.NotificationManager.clear();
    };

    // Initialize when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            // Ready
        });
    }
})();
