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

    // Security helpers
    const security = {
        getCSRFTokenFromMeta() {
            const meta = document.querySelector('meta[name="csrf-token"]');
            return meta ? meta.getAttribute('content') : null;
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

            // Check if there's already a server-rendered error (without data-js-error)
            const existingServerError = formField.querySelector('.help.is-danger:not([data-js-error])');
            if (existingServerError) {
                // Server error exists, don't add JS error (avoid double messages)
                field.classList.add('is-danger');
                return;
            }

            // Remove existing JS-generated error
            this.clearFieldError(field);

            // Add error styling
            field.classList.add('is-danger');
            
            // Create error message (marked as JS-generated)
            const errorDiv = document.createElement('p');
            errorDiv.className = 'help is-danger';
            errorDiv.setAttribute('data-js-error', 'true');
            errorDiv.textContent = message;
            errorDiv.setAttribute('role', 'alert');
            errorDiv.setAttribute('aria-live', 'polite');
            
            formField.appendChild(errorDiv);
        },

        clearFieldError(field) {
            const formField = field.closest('.field');
            if (!formField) return;

            field.classList.remove('is-danger');
            // Only remove JS-generated errors, preserve server-side errors
            const jsErrorDiv = formField.querySelector('.help.is-danger[data-js-error]');
            if (jsErrorDiv) {
                jsErrorDiv.remove();
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

    // Title management
    const titleManager = {
        init() {
            const pageTitle = document.documentElement.dataset.pageTitle;
            const baseTitle = document.documentElement.dataset.baseTitle || document.title;
            
            if (pageTitle && document.title === baseTitle) {
                document.title = pageTitle + ' - ' + baseTitle;
            }
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
        titleManager.init();
        formValidation.init();
        eventHandlers.init();
        progressiveEnhancement.init();
        errorHandler.init();

        // Expose security helpers globally for other modules
        if (!window.PVMSS) {
            window.PVMSS = {};
        }
        if (!window.PVMSS.security) {
            window.PVMSS.security = {};
        }
        window.PVMSS.security.getCSRFTokenFromMeta = security.getCSRFTokenFromMeta;

        // Add class to indicate JS is available
        document.documentElement.classList.add('js');
    }

    init();
})();

