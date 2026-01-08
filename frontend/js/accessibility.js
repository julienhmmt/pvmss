/**
 * Accessibility and Progressive Enhancement JavaScript - Optimized
 * Handles essential accessibility features only
 */

(function() {
    'use strict';

    // Utility functions
    const utils = {
        exists: (selector) => document.querySelector(selector) !== null,
        getByData: (attr, value) => document.querySelector(`[data-${attr}="${value}"]`),
        on: (event, selector, handler) => {
            document.addEventListener(event, (e) => {
                if (e.target.matches(selector)) {
                    handler.call(e.target, e);
                }
            });
        },
        toggleVisibility: (element, show) => {
            if (!element) return;
            element.style.display = show ? '' : 'none';
            element.setAttribute('aria-hidden', !show);
        }
    };

    // Security helpers
    const security = {
        getCSRFToken() {
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
            forms.forEach(form => this.enhanceForm(form));
        },

        enhanceForm(form) {
            // Add real-time validation to required fields
            const requiredFields = form.querySelectorAll('[required]');
            requiredFields.forEach(field => {
                field.addEventListener('blur', () => this.validateField(field));
                field.addEventListener('input', () => this.clearFieldError(field));
            });

            // Handle form submission
            form.addEventListener('submit', (e) => {
                if (!this.validateForm(form)) {
                    e.preventDefault();
                    this.showFormError(form, 'Please correct the errors below');
                }
            });
        },

        validateField(field) {
            const isValid = field.checkValidity();
            field.classList.toggle('is-danger', !isValid);
            
            let errorMsg = field.parentNode.querySelector('.help.is-danger');
            if (!isValid && !errorMsg) {
                errorMsg = document.createElement('p');
                errorMsg.className = 'help is-danger';
                errorMsg.textContent = field.validationMessage || 'This field is required';
                field.parentNode.appendChild(errorMsg);
            } else if (isValid && errorMsg) {
                errorMsg.remove();
            }
            
            return isValid;
        },

        clearFieldError(field) {
            field.classList.remove('is-danger');
            const errorMsg = field.parentNode.querySelector('.help.is-danger');
            if (errorMsg) errorMsg.remove();
        },

        validateForm(form) {
            const requiredFields = form.querySelectorAll('[required]');
            let isValid = true;
            
            requiredFields.forEach(field => {
                if (!this.validateField(field)) {
                    isValid = false;
                }
            });
            
            return isValid;
        },

        showFormError(form, message) {
            let errorDiv = form.querySelector('.form-error');
            if (!errorDiv) {
                errorDiv = document.createElement('div');
                errorDiv.className = 'notification is-danger form-error';
                form.insertBefore(errorDiv, form.firstChild);
            }
            errorDiv.textContent = message;
            errorDiv.focus();
        },

        addRealTimeValidation() {
            // Add input validation for specific patterns
            const patternFields = document.querySelectorAll('input[pattern]');
            patternFields.forEach(field => {
                field.addEventListener('input', () => {
                    const pattern = new RegExp(field.pattern);
                    const isValid = pattern.test(field.value);
                    field.classList.toggle('is-danger', !isValid && field.value.length > 0);
                });
            });
        }
    };

    // Focus management
    const focusManagement = {
        init() {
            this.addFocusTrapping();
        },

        addFocusTrapping() {
            // Trap focus within modals
            const modals = document.querySelectorAll('.modal');
            modals.forEach(modal => {
                modal.addEventListener('keydown', (e) => {
                    if (e.key === 'Tab') {
                        this.trapFocus(e, modal);
                    }
                });
            });
        },

        trapFocus(e, container) {
            const focusableElements = container.querySelectorAll(
                'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
            );
            const firstElement = focusableElements[0];
            const lastElement = focusableElements[focusableElements.length - 1];

            if (e.shiftKey) {
                if (document.activeElement === firstElement) {
                    lastElement.focus();
                    e.preventDefault();
                }
            } else {
                if (document.activeElement === lastElement) {
                    firstElement.focus();
                    e.preventDefault();
                }
            }
        }
    };

    // Initialize everything when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            formValidation.init();
            focusManagement.init();
        });
    } else {
        formValidation.init();
        focusManagement.init();
    }

    // Export for testing
    window.Accessibility = {
        utils,
        security,
        formValidation,
        focusManagement
    };
})();
