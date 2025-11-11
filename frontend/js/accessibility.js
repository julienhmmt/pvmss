/**
 * Accessibility and Progressive Enhancement JavaScript
 * Handles event delegation, form validation, and progressive enhancement
 */

(function() {
    'use strict';

    // Utility functions
    const utils = {
        // Check if element exists
        exists: (selector) => document.querySelector(selector) !== null,
        
        // Get element by data attribute
        getByData: (attr, value) => document.querySelector(`[data-${attr}="${value}"]`),
        
        // Add event listener with delegation
        on: (event, selector, handler) => {
            document.addEventListener(event, (e) => {
                if (e.target.matches(selector)) {
                    handler.call(e.target, e);
                }
            });
        },
        
        // Show/hide element with accessibility
        toggleVisibility: (element, show) => {
            if (!element) return;
            element.style.display = show ? '' : 'none';
            element.setAttribute('aria-hidden', !show);
        }
    };

    const notifications = {
        storagePrefix: 'pvmss.notification.',
        timers: new WeakMap(),

        init() {
            const elements = document.querySelectorAll('[data-notification="true"]');
            elements.forEach((element) => this.enhance(element));

            if (!window.PVMSSNotifications) {
                window.PVMSSNotifications = {
                    enhance: (el) => this.enhance(el),
                    dismiss: (el, options) => this.dismiss(el, options || {}),
                };
            }
        },

        enhance(element) {
            if (!element || element.dataset.notificationEnhanced === 'true') {
                return;
            }

            const persistKey = element.dataset.notificationKey;
            if (persistKey && this.isPersisted(persistKey)) {
                element.remove();
                return;
            }

            element.dataset.notificationEnhanced = 'true';

            if (persistKey) {
                const onDismissPersist = () => {
                    this.persistDismiss(persistKey);
                };
                element.addEventListener('notification:dismiss', onDismissPersist, { once: true });
            }

            if (element.dataset.autoDismiss === 'true') {
                this.setupAutoDismiss(element);
            }
        },

        getStorage() {
            try {
                return window.localStorage;
            } catch (error) {
                return null;
            }
        },

        isPersisted(key) {
            const storage = this.getStorage();
            if (!storage) {
                return false;
            }
            return storage.getItem(this.storagePrefix + key) === 'dismissed';
        },

        persistDismiss(key) {
            const storage = this.getStorage();
            if (!storage) {
                return;
            }
            storage.setItem(this.storagePrefix + key, 'dismissed');
        },

        dismiss(element, options = {}) {
            if (!element || element.classList.contains('is-dismissed')) {
                return;
            }

            const { announce = true, announcement } = options;

            const timer = this.timers.get(element);
            if (timer && typeof timer.cleanup === 'function') {
                timer.cleanup();
            }

            element.classList.add('is-dismissed');
            element.setAttribute('data-auto-dismiss-state', 'dismissed');

            element.dispatchEvent(new CustomEvent('notification:dismiss'));

            if (announce) {
                const message = announcement || element.dataset.dismissAnnouncement || 'Notification dismissed';
                this.announce(message);
            }

            const finalize = () => {
                element.style.display = 'none';
                element.setAttribute('aria-hidden', 'true');
            };

            const transitionHandler = (event) => {
                if (event.target === element && event.propertyName === 'opacity') {
                    element.removeEventListener('transitionend', transitionHandler);
                    finalize();
                }
            };

            element.addEventListener('transitionend', transitionHandler);
            window.setTimeout(() => {
                element.removeEventListener('transitionend', transitionHandler);
                finalize();
            }, 250);
        },

        announce(message) {
            if (!message) {
                return;
            }
            const announcement = document.createElement('div');
            announcement.className = 'visually-hidden';
            announcement.setAttribute('aria-live', 'polite');
            announcement.textContent = message;
            document.body.appendChild(announcement);
            window.setTimeout(() => announcement.remove(), 1000);
        },

        setupAutoDismiss(element) {
            const initialDelay = parseInt(element.dataset.autoDismissDelay, 10) || 6000;
            const state = {
                remaining: initialDelay,
                timerId: null,
                lastStart: null,
            };

            const tick = () => {
                state.timerId = null;
                this.dismiss(element, { announce: false });
            };

            const start = () => {
                if (state.timerId || element.classList.contains('is-dismissed')) {
                    return;
                }
                state.lastStart = Date.now();
                state.timerId = window.setTimeout(tick, state.remaining);
                element.setAttribute('data-auto-dismiss-state', 'running');
            };

            const pause = () => {
                if (state.timerId) {
                    window.clearTimeout(state.timerId);
                    state.timerId = null;
                    state.remaining -= Date.now() - (state.lastStart || Date.now());
                    if (state.remaining < 0) {
                        state.remaining = 0;
                    }
                }
                element.setAttribute('data-auto-dismiss-state', 'paused');
            };

            const resume = () => {
                if (state.timerId || element.classList.contains('is-dismissed')) {
                    return;
                }
                if (state.remaining <= 0) {
                    tick();
                } else {
                    start();
                }
            };

            const cleanup = () => {
                pause();
                element.removeEventListener('mouseenter', pause);
                element.removeEventListener('mouseleave', resume);
                element.removeEventListener('focusin', pause);
                element.removeEventListener('focusout', resume);
            };

            element.addEventListener('mouseenter', pause);
            element.addEventListener('mouseleave', resume);
            element.addEventListener('focusin', pause);
            element.addEventListener('focusout', resume);
            element.addEventListener('notification:dismiss', cleanup, { once: true });

            this.timers.set(element, { pause, resume, cleanup });
            start();
        }
    };

    // Form validation enhancements
    const formValidation = {
        init() {
            this.addCustomValidation();
            this.addRealTimeValidation();
        },

        addCustomValidation() {
            const forms = document.querySelectorAll('form[data-validate="enhanced"]');
            forms.forEach(form => {
                form.addEventListener('submit', (e) => {
                    if (!form.checkValidity()) {
                        e.preventDefault();
                        this.showValidationErrors(form);
                    }
                });
            });
        },

        addRealTimeValidation() {
            const inputs = document.querySelectorAll('input[required], select[required], textarea[required]');
            inputs.forEach(input => {
                input.addEventListener('blur', () => this.validateField(input));
                input.addEventListener('input', () => this.clearFieldError(input));
            });
        },

        validateField(field) {
            if (!field.validity.valid) {
                this.showFieldError(field, field.validationMessage);
            } else {
                this.clearFieldError(field);
            }
        },

        showFieldError(field, message) {
            const formField = field.closest('.field');
            if (!formField) return;

            // Remove existing error
            this.clearFieldError(field);

            // Add error styling
            field.classList.add('is-danger');
            
            // Create error message
            const errorDiv = document.createElement('p');
            errorDiv.className = 'help is-danger';
            errorDiv.textContent = message;
            errorDiv.setAttribute('role', 'alert');
            errorDiv.setAttribute('aria-live', 'polite');
            
            formField.appendChild(errorDiv);
        },

        clearFieldError(field) {
            const formField = field.closest('.field');
            if (!formField) return;

            field.classList.remove('is-danger');
            const errorDiv = formField.querySelector('.help.is-danger');
            if (errorDiv) {
                errorDiv.remove();
            }
        },

        showValidationErrors(form) {
            const invalidFields = form.querySelectorAll(':invalid');
            invalidFields.forEach(field => this.validateField(field));
        }
    };

    // Event delegation for data-action attributes
    const eventHandlers = {
        init() {
            this.bindEvents();
        },

        bindEvents() {
            // Dismiss notifications
            utils.on('click', '[data-action="dismiss"]', function(e) {
                e.preventDefault();
                const target = this.dataset.target;
                let element = null;

                if (target) {
                    if (target.startsWith('#')) {
                        element = document.querySelector(target);
                    } else {
                        element = document.getElementById(target);
                    }
                }

                if (!element) {
                    element = this.closest('[data-notification="true"]') || this.closest('.notification');
                }

                if (element && element.dataset && element.dataset.notification === 'true') {
                    notifications.dismiss(element);
                } else if (element) {
                    utils.toggleVisibility(element, false);
                    notifications.announce('Notification dismissed');
                }
            });

            // Back button functionality
            utils.on('click', '[data-action="back"]', function(e) {
                e.preventDefault();
                if (window.history.length > 1) {
                    window.history.back();
                } else {
                    window.location.href = '/';
                }
            });

            // Form submission enhancement
            utils.on('submit', 'form[data-enhance="true"]', function(e) {
                const submitBtn = this.querySelector('[type="submit"]');
                if (submitBtn && this.checkValidity()) {
                    submitBtn.disabled = true;
                    submitBtn.classList.add('is-loading');
                }
            });

            // Checkbox enhancement for tags
            utils.on('click', '.tag-checkbox-label', function(e) {
                const checkbox = this.querySelector('input[type="checkbox"]');
                if (checkbox) {
                    checkbox.checked = !checkbox.checked;
                    this.classList.toggle('is-selected', checkbox.checked);
                }
            });
        }
    };

    // Progressive enhancement for forms
    const progressiveEnhancement = {
        init() {
            this.enhanceForms();
            this.enhanceInteractiveElements();
        },

        enhanceForms() {
            const forms = document.querySelectorAll('form');
            forms.forEach(form => {
                // Add data attributes for JavaScript enhancement
                form.setAttribute('data-enhance', 'true');
                form.setAttribute('novalidate', ''); // Let JS handle validation
                form.setAttribute('data-validate', 'enhanced');
            });
        },

        enhanceInteractiveElements() {
            // Enhance tag checkboxes for better UX
            const tagLabels = document.querySelectorAll('label:has(input[type="checkbox"])');
            tagLabels.forEach(label => {
                label.classList.add('tag-checkbox-label');
                label.setAttribute('tabindex', '0');
                label.setAttribute('role', 'checkbox');
                
                const checkbox = label.querySelector('input[type="checkbox"]');
                if (checkbox) {
                    label.setAttribute('aria-checked', checkbox.checked);
                    
                    // Keyboard support
                    label.addEventListener('keydown', (e) => {
                        if (e.key === ' ' || e.key === 'Enter') {
                            e.preventDefault();
                            checkbox.checked = !checkbox.checked;
                            label.setAttribute('aria-checked', checkbox.checked);
                            label.classList.toggle('is-selected', checkbox.checked);
                        }
                    });
                }
            });
        }
    };

    // Error handling
    const errorHandler = {
        init() {
            window.addEventListener('error', (e) => {
                console.error('JavaScript error:', e.error);
                this.showUserFriendlyError('An unexpected error occurred. Please try again.');
            });

            window.addEventListener('unhandledrejection', (e) => {
                console.error('Unhandled promise rejection:', e.reason);
                this.showUserFriendlyError('A network error occurred. Please check your connection.');
            });
        },

        showUserFriendlyError(message) {
            const errorDiv = document.createElement('div');
            errorDiv.className = 'notification is-danger is-light';
            errorDiv.setAttribute('role', 'alert');
            errorDiv.innerHTML = `
                <div class="level is-mobile">
                    <div class="level-left">
                        <div class="level-item">
                            <span class="icon has-text-danger">
                                <i class="fas fa-exclamation-triangle"></i>
                            </span>
                        </div>
                        <div class="level-item">
                            <span>${message}</span>
                        </div>
                    </div>
                    <div class="level-right">
                        <button class="delete" data-action="dismiss" aria-label="Dismiss error message"></button>
                    </div>
                </div>
            `;
            
            const main = document.querySelector('main') || document.body;
            main.insertBefore(errorDiv, main.firstChild);
        }
    };

    // Initialize everything when DOM is ready
    function init() {
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', initApp);
        } else {
            initApp();
        }
    }

    function initApp() {
        formValidation.init();
        eventHandlers.init();
        progressiveEnhancement.init();
        errorHandler.init();
        notifications.init();

        // Add class to indicate JS is available
        document.documentElement.classList.add('js');
    }

    init();
})();

